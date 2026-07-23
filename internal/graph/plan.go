package graph

import (
	"errors"
	"fmt"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"

	"github.com/runmedev/owl/internal/store"
)

type plannedQuery struct {
	Query string
	Vars  map[string]interface{}
	Path  []string
}

func planStateEnvelopeQuery(records []store.OperationRecord) (plannedQuery, error) {
	if len(records) == 0 {
		return plannedQuery{}, errors.New("operation plan is empty")
	}

	varDefs := make([]*ast.VariableDefinition, 0, len(records))
	vars := make(map[string]interface{}, len(records))
	path := make([]string, 0, len(records)+4)
	root := ast.NewSelectionSet(&ast.SelectionSet{})
	current := root

	for index, record := range records {
		next := ast.NewSelectionSet(&ast.SelectionSet{})
		switch record.Kind {
		case store.OperationRecordLoad:
			name := fmt.Sprintf("load_%d", index)
			varDefs = append(varDefs, variableDefinition(name, nonNull(namedType("LoadInput"))))
			vars[name] = marshalInput(record.Load)
			current.Selections = append(current.Selections, field("load", []*ast.Argument{
				argument("input", variable(name)),
			}, next))
			path = append(path, "load")
		case store.OperationRecordUpdate:
			name := fmt.Sprintf("update_%d", index)
			varDefs = append(varDefs, variableDefinition(name, namedType("DotenvInput")))
			vars[name] = marshalDotenvInput(store.LoadInput{
				DotenvSource: record.Update.Source,
				Dotenv:       record.Update.Dotenv,
			})
			current.Selections = append(current.Selections, field("update", []*ast.Argument{
				argument("dotenv", variable(name)),
			}, next))
			path = append(path, "update")
		case store.OperationRecordDelete:
			name := fmt.Sprintf("delete_%d", index)
			varDefs = append(varDefs, variableDefinition(name, list(nonNull(namedType("String")))))
			vars[name] = append([]string{}, record.Delete.Keys...)
			current.Selections = append(current.Selections, field("delete", []*ast.Argument{
				argument("keys", variable(name)),
			}, next))
			path = append(path, "delete")
		default:
			return plannedQuery{}, fmt.Errorf("unsupported operation record kind %q", record.Kind)
		}
		current = next
	}

	current.Selections = append(current.Selections, stateEnvelopeTerminal())
	path = append(path, "normalize", "validate", "state", "envelope")

	doc := ast.NewDocument(&ast.Document{Definitions: []ast.Node{
		ast.NewOperationDefinition(&ast.OperationDefinition{
			Operation:           "query",
			Name:                name("OwlPlannedStateEnvelope"),
			VariableDefinitions: varDefs,
			SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
				field("Environment", nil, root),
			}}),
		}),
	}})

	printed := printer.Print(doc)
	query, ok := printed.(string)
	if !ok {
		return plannedQuery{}, errors.New("ast printer returned unknown type")
	}
	return plannedQuery{Query: query, Vars: vars, Path: path}, nil
}

func stateEnvelopeTerminal() *ast.Field {
	return field("normalize", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
		field("validate", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
			field("state", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
				field("envelope", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
					field("modelVersion", nil, nil),
					field("state", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
						field("values", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
							field("field", nil, fieldRefSelection()),
							field("original", nil, nil),
							field("resolved", nil, nil),
							field("visibility", nil, nil),
							field("sensitivity", nil, nil),
							field("exposure", nil, nil),
							field("origin", nil, sourceSelection()),
							field("source", nil, sourceSelection()),
						}})),
						field("bindings", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
							field("id", nil, nil),
							field("field", nil, fieldRefSelection()),
							field("projection", nil, nil),
							field("key", nil, nil),
							field("description", nil, nil),
							field("source", nil, sourceSelection()),
							field("origin", nil, sourceSelection()),
							field("confidence", nil, nil),
							field("explicit", nil, nil),
							field("preserveKey", nil, nil),
						}})),
						field("diagnostics", nil, diagnosticSelection()),
					}})),
					field("provenance", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
						field("sources", nil, sourceSelection()),
					}})),
				}})),
			}})),
		}})),
	}}))
}

func fieldRefSelection() *ast.SelectionSet {
	return ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
		field("typeID", nil, nil),
		field("instance", nil, nil),
		field("field", nil, nil),
	}})
}

func sourceSelection() *ast.SelectionSet {
	return ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
		field("name", nil, nil),
		field("kind", nil, nil),
	}})
}

func diagnosticSelection() *ast.SelectionSet {
	return ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
		field("severity", nil, nil),
		field("code", nil, nil),
		field("message", nil, nil),
		field("key", nil, nil),
		field("field", nil, nil),
	}})
}

func field(fieldName string, args []*ast.Argument, selection *ast.SelectionSet) *ast.Field {
	return ast.NewField(&ast.Field{
		Name:         name(fieldName),
		Arguments:    args,
		Directives:   []*ast.Directive{},
		SelectionSet: selection,
	})
}

func argument(argumentName string, value ast.Value) *ast.Argument {
	return ast.NewArgument(&ast.Argument{
		Name:  name(argumentName),
		Value: value,
	})
}

func variable(variableName string) *ast.Variable {
	return ast.NewVariable(&ast.Variable{Name: name(variableName)})
}

func variableDefinition(variableName string, typ ast.Type) *ast.VariableDefinition {
	return ast.NewVariableDefinition(&ast.VariableDefinition{
		Variable: variable(variableName),
		Type:     typ,
	})
}

func name(value string) *ast.Name {
	return ast.NewName(&ast.Name{Value: value})
}

func namedType(typeName string) ast.Type {
	return ast.NewNamed(&ast.Named{Name: name(typeName)})
}

func nonNull(typ ast.Type) ast.Type {
	return ast.NewNonNull(&ast.NonNull{Type: typ})
}

func list(typ ast.Type) ast.Type {
	return ast.NewList(&ast.List{Type: typ})
}
