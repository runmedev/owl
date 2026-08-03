package graph

import (
	"errors"
	"fmt"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"

	"github.com/runmedev/owl/internal/model"
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
			input := record.Load
			input.Timestamp = record.Timestamp
			vars[name] = marshalInput(input)
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
				Timestamp:    record.Timestamp,
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
		case store.OperationRecordResolverAttempt:
			name := fmt.Sprintf("resolverAttempt_%d", index)
			varDefs = append(varDefs, variableDefinition(name, nonNull(namedType("ResolverAttemptInput"))))
			vars[name] = marshalResolverAttempt(record.ResolverAttempt)
			current.Selections = append(current.Selections, field("recordResolverAttempt", []*ast.Argument{
				argument("attempt", variable(name)),
			}, next))
			path = append(path, "recordResolverAttempt")
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
							field("createdAt", nil, nil),
							field("updatedAt", nil, nil),
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
							field("order", nil, nil),
							field("preserveKey", nil, nil),
							field("required", nil, nil),
						}})),
						field("resolverAttempts", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
							field("id", nil, nil),
							field("resolverID", nil, nil),
							field("field", nil, fieldRefSelection()),
							field("projectionKey", nil, nil),
							field("outcome", nil, nil),
							field("message", nil, nil),
							field("source", nil, sourceSelection()),
							field("startedAt", nil, nil),
							field("finishedAt", nil, nil),
							field("diagnostics", nil, diagnosticSelection()),
						}})),
						field("unresolvedFrontier", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
							field("needs", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
								field("id", nil, nil),
								field("field", nil, fieldRefSelection()),
								field("projectionKey", nil, nil),
								field("required", nil, nil),
								field("blocking", nil, nil),
								field("reason", nil, nil),
								field("description", nil, nil),
								field("sensitivity", nil, nil),
								field("exposure", nil, nil),
								field("source", nil, sourceSelection()),
								field("origin", nil, sourceSelection()),
								field("resolverAttemptIDs", nil, nil),
							}})),
						}})),
						field("diagnostics", nil, diagnosticSelection()),
					}})),
					field("provenance", nil, ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
						field("sources", nil, sourceSelection()),
						field("operations", nil, operationMetadataSelection()),
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
		field("details", nil, nil),
		field("key", nil, nil),
		field("field", nil, nil),
		field("owner", nil, nil),
	}})
}

func marshalResolverAttempt(attempt model.ResolverAttempt) map[string]interface{} {
	result := map[string]interface{}{
		"id":            string(attempt.ID),
		"resolverID":    string(attempt.ResolverID),
		"field":         map[string]interface{}{"typeID": string(attempt.FieldRef.TypeID), "instance": attempt.FieldRef.Instance, "field": attempt.FieldRef.Field},
		"projectionKey": string(attempt.ProjectionKey),
		"outcome":       string(attempt.Outcome),
		"message":       attempt.Message,
		"startedAt":     timeString(attempt.StartedAt),
		"finishedAt":    timeString(attempt.FinishedAt),
	}
	if attempt.Source.Name != "" || attempt.Source.Kind != "" {
		result["source"] = map[string]interface{}{"name": attempt.Source.Name, "kind": attempt.Source.Kind}
	}
	diagnostics := make([]map[string]interface{}, 0, len(attempt.Diagnostics))
	for _, diagnostic := range attempt.Diagnostics {
		diagnostics = append(diagnostics, map[string]interface{}{
			"severity": string(diagnostic.Severity),
			"code":     diagnostic.Code,
			"message":  diagnostic.Message,
			"details":  append([]string{}, diagnostic.Details...),
			"key":      diagnostic.Key,
			"field":    map[string]interface{}{"typeID": string(diagnostic.FieldRef.TypeID), "instance": diagnostic.FieldRef.Instance, "field": diagnostic.FieldRef.Field},
			"owner":    string(diagnostic.Owner),
		})
	}
	result["diagnostics"] = diagnostics
	return result
}

func operationMetadataSelection() *ast.SelectionSet {
	return ast.NewSelectionSet(&ast.SelectionSet{Selections: []ast.Selection{
		field("id", nil, nil),
		field("kind", nil, nil),
		field("timestamp", nil, nil),
		field("actor", nil, nil),
		field("source", nil, sourceSelection()),
		field("projection", nil, nil),
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
