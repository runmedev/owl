package graph

import (
	"context"
	"sort"

	"github.com/graphql-go/graphql"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/store"
)

func (r *Runtime) newSchema() (graphql.Schema, error) {
	sourceInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SourceInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"name": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"kind": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	fieldRefInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "FieldRefInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"typeID":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"instance": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"field":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	dotenvVariableInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "DotenvVariableInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"key":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"value": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	dotenvInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "DotenvInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"source":    &graphql.InputObjectFieldConfig{Type: sourceInput},
			"variables": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(dotenvVariableInput))},
		},
	})
	envBindingInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "EnvBindingInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"field":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(fieldRefInput)},
			"key":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"projection":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"required":    &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"description": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"source":      &graphql.InputObjectFieldConfig{Type: sourceInput},
		},
	})
	envContractInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "EnvContractInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"source":     &graphql.InputObjectFieldConfig{Type: sourceInput},
			"projection": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"bindings":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(envBindingInput))},
		},
	})
	loadInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "LoadInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"dotenv":    &graphql.InputObjectFieldConfig{Type: dotenvInput},
			"contracts": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(envContractInput))},
		},
	})

	diagnosticType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Diagnostic",
		Fields: graphql.Fields{
			"severity": &graphql.Field{Type: graphql.String},
			"code":     &graphql.Field{Type: graphql.String},
			"message":  &graphql.Field{Type: graphql.String},
			"key":      &graphql.Field{Type: graphql.String},
			"field": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					diagnostic := p.Source.(model.Diagnostic)
					if diagnostic.FieldRef.TypeID == "" {
						return "", nil
					}
					return diagnostic.FieldRef.String(), nil
				},
			},
		},
	})
	checkType := graphql.NewObject(graphql.ObjectConfig{
		Name: "CheckResult",
		Fields: graphql.Fields{
			"ok":          &graphql.Field{Type: graphql.Boolean},
			"diagnostics": &graphql.Field{Type: graphql.NewList(diagnosticType)},
		},
	})
	snapshotItemType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SnapshotItem",
		Fields: graphql.Fields{
			"name":          &graphql.Field{Type: graphql.String},
			"value":         &graphql.Field{Type: graphql.String},
			"originalValue": &graphql.Field{Type: graphql.String},
			"type": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return string(item.Type), nil
				},
			},
			"field": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Field.String(), nil
				},
			},
			"fieldTypeID": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return string(item.Field.TypeID), nil
				},
			},
			"fieldInstance": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Field.Instance, nil
				},
			},
			"fieldName": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Field.Field, nil
				},
			},
			"source": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Source.Name, nil
				},
			},
			"origin": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Origin.Name, nil
				},
			},
			"status": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return string(item.Status), nil
				},
			},
			"description": &graphql.Field{Type: graphql.String},
			"diagnostics": &graphql.Field{Type: graphql.NewList(diagnosticType)},
		},
	})

	var environmentType *graphql.Object

	renderType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Render",
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"snapshot": &graphql.Field{
					Type: graphql.NewList(snapshotItemType),
					Args: graphql.FieldConfigArgument{
						"reveal": &graphql.ArgumentConfig{Type: graphql.Boolean, DefaultValue: false},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						reveal, _ := p.Args["reveal"].(bool)
						return store.NewState(gctx.State, gctx.Types).Snapshot(store.SnapshotPolicy{Reveal: reveal})
					},
				},
				"dotenv": &graphql.Field{
					Type: graphql.NewList(graphql.String),
					Args: graphql.FieldConfigArgument{
						"insecure": &graphql.ArgumentConfig{Type: graphql.Boolean, DefaultValue: false},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						insecure, _ := p.Args["insecure"].(bool)
						return store.NewState(gctx.State, gctx.Types).Source(store.SourcePolicy{Insecure: insecure})
					},
				},
				"check": &graphql.Field{
					Type: checkType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						check := store.NewState(gctx.State, gctx.Types).Check()
						check.Diagnostics = sortedDiagnostics(check.Diagnostics)
						return check, nil
					},
				},
			}
		}),
	})

	environmentType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Environment",
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"load": &graphql.Field{
					Type: environmentType,
					Args: graphql.FieldConfigArgument{
						"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(loadInput)},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						input := decodeLoadInput(p.Args["input"].(map[string]interface{}))
						gctx := p.Source.(Context)
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.LoadOperation{Input: input})
						if err != nil {
							return nil, err
						}
						return Context{State: state, Types: gctx.Types}, nil
					},
				},
				"normalize": &graphql.Field{
					Type: environmentType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.NormalizeOperation{})
						if err != nil {
							return nil, err
						}
						return Context{State: state, Types: gctx.Types}, nil
					},
				},
				"validate": &graphql.Field{
					Type: environmentType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.ValidateOperation{Types: gctx.Types})
						if err != nil {
							return nil, err
						}
						return Context{State: state, Types: gctx.Types}, nil
					},
				},
				"render": &graphql.Field{
					Type: renderType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
			}
		}),
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"Environment": &graphql.Field{
				Type: environmentType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return Context{State: model.NewEffectiveState(), Types: r.types}, nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{Query: queryType})
}

func contextFromParams(p graphql.ResolveParams) context.Context {
	if p.Context == nil {
		return context.Background()
	}
	return p.Context
}

func decodeLoadInput(raw map[string]interface{}) store.LoadInput {
	var input store.LoadInput
	if dotenvRaw, ok := raw["dotenv"].(map[string]interface{}); ok {
		input.DotenvSource = decodeSource(dotenvRaw["source"])
		for _, item := range decodeList(dotenvRaw["variables"]) {
			variable, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			input.Dotenv = append(input.Dotenv, store.DotenvVariable{
				Key:   stringValue(variable["key"]),
				Value: stringValue(variable["value"]),
			})
		}
	}
	for _, item := range decodeList(raw["contracts"]) {
		contractRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		contract := store.EnvContract{
			Source:     decodeSource(contractRaw["source"]),
			Projection: model.ProjectionID(stringValue(contractRaw["projection"])),
		}
		for _, bindingItem := range decodeList(contractRaw["bindings"]) {
			bindingRaw, ok := bindingItem.(map[string]interface{})
			if !ok {
				continue
			}
			contract.Bindings = append(contract.Bindings, store.EnvBinding{
				FieldRef:    decodeFieldRef(bindingRaw["field"]),
				Key:         stringValue(bindingRaw["key"]),
				Projection:  model.ProjectionID(stringValue(bindingRaw["projection"])),
				Required:    boolValue(bindingRaw["required"]),
				Description: stringValue(bindingRaw["description"]),
				Source:      decodeSource(bindingRaw["source"]),
			})
		}
		input.Contracts = append(input.Contracts, contract)
	}
	sort.SliceStable(input.Contracts, func(i, j int) bool {
		return input.Contracts[i].Source.Name < input.Contracts[j].Source.Name
	})
	return input
}

func decodeSource(raw interface{}) model.Source {
	source, ok := raw.(map[string]interface{})
	if !ok {
		return model.Source{}
	}
	return model.Source{Name: stringValue(source["name"]), Kind: stringValue(source["kind"])}
}

func decodeFieldRef(raw interface{}) model.FieldRef {
	field, ok := raw.(map[string]interface{})
	if !ok {
		return model.FieldRef{}
	}
	return model.FieldRef{
		TypeID:   model.TypeID(stringValue(field["typeID"])),
		Instance: stringValue(field["instance"]),
		Field:    stringValue(field["field"]),
	}
}

func decodeList(raw interface{}) []interface{} {
	if raw == nil {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	return items
}

func stringValue(raw interface{}) string {
	value, _ := raw.(string)
	return value
}

func boolValue(raw interface{}) bool {
	value, _ := raw.(bool)
	return value
}
