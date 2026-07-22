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
	diagnosticInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "DiagnosticInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"severity": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"code":     &graphql.InputObjectFieldConfig{Type: graphql.String},
			"message":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"key":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"field":    &graphql.InputObjectFieldConfig{Type: fieldRefInput},
		},
	})
	stateValueInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StateValueInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"field":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(fieldRefInput)},
			"original":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"resolved":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"visibility":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"sensitivity": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"exposure":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"origin":      &graphql.InputObjectFieldConfig{Type: sourceInput},
			"source":      &graphql.InputObjectFieldConfig{Type: sourceInput},
		},
	})
	stateBindingInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StateBindingInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"id":          &graphql.InputObjectFieldConfig{Type: graphql.String},
			"field":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(fieldRefInput)},
			"projection":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"key":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"description": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"source":      &graphql.InputObjectFieldConfig{Type: sourceInput},
			"origin":      &graphql.InputObjectFieldConfig{Type: sourceInput},
			"confidence":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"explicit":    &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"preserveKey": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		},
	})
	effectiveStateInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "EffectiveStateInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"values":      &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(stateValueInput))},
			"bindings":    &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(stateBindingInput))},
			"diagnostics": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(diagnosticInput))},
		},
	})
	stateEnvelopeInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StateEnvelopeInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"modelVersion": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"state":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(effectiveStateInput)},
		},
	})
	loadInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "LoadInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"dotenv":    &graphql.InputObjectFieldConfig{Type: dotenvInput},
			"contracts": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(envContractInput))},
			"envelope":  &graphql.InputObjectFieldConfig{Type: stateEnvelopeInput},
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
					var fieldRef model.FieldRef
					switch diagnostic := p.Source.(type) {
					case model.Diagnostic:
						fieldRef = diagnostic.FieldRef
					case map[string]interface{}:
						fieldRef = decodeFieldRef(diagnostic["field"])
					}
					if fieldRef.TypeID == "" {
						return "", nil
					}
					return fieldRef.String(), nil
				},
			},
		},
	})
	sourceType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Source",
		Fields: graphql.Fields{
			"name": &graphql.Field{Type: graphql.String},
			"kind": &graphql.Field{Type: graphql.String},
		},
	})
	fieldRefType := graphql.NewObject(graphql.ObjectConfig{
		Name: "FieldRef",
		Fields: graphql.Fields{
			"typeID":   &graphql.Field{Type: graphql.String},
			"instance": &graphql.Field{Type: graphql.String},
			"field":    &graphql.Field{Type: graphql.String},
			"display": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					switch ref := p.Source.(type) {
					case model.FieldRef:
						return ref.String(), nil
					case map[string]interface{}:
						return decodeFieldRef(ref).String(), nil
					default:
						return "", nil
					}
				},
			},
		},
	})
	stateValueType := graphql.NewObject(graphql.ObjectConfig{
		Name: "StateValue",
		Fields: graphql.Fields{
			"field":       &graphql.Field{Type: fieldRefType},
			"original":    &graphql.Field{Type: graphql.String},
			"resolved":    &graphql.Field{Type: graphql.String},
			"visibility":  &graphql.Field{Type: graphql.String},
			"sensitivity": &graphql.Field{Type: graphql.String},
			"exposure":    &graphql.Field{Type: graphql.String},
			"origin":      &graphql.Field{Type: sourceType},
			"source":      &graphql.Field{Type: sourceType},
		},
	})
	stateBindingType := graphql.NewObject(graphql.ObjectConfig{
		Name: "StateBinding",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.String},
			"field":       &graphql.Field{Type: fieldRefType},
			"projection":  &graphql.Field{Type: graphql.String},
			"key":         &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"source":      &graphql.Field{Type: sourceType},
			"origin":      &graphql.Field{Type: sourceType},
			"confidence":  &graphql.Field{Type: graphql.String},
			"explicit":    &graphql.Field{Type: graphql.Boolean},
			"preserveKey": &graphql.Field{Type: graphql.Boolean},
		},
	})
	effectiveStateType := graphql.NewObject(graphql.ObjectConfig{
		Name: "EffectiveState",
		Fields: graphql.Fields{
			"values":      &graphql.Field{Type: graphql.NewList(stateValueType)},
			"bindings":    &graphql.Field{Type: graphql.NewList(stateBindingType)},
			"diagnostics": &graphql.Field{Type: graphql.NewList(diagnosticType)},
		},
	})
	stateProvenanceType := graphql.NewObject(graphql.ObjectConfig{
		Name: "StateProvenance",
		Fields: graphql.Fields{
			"sources": &graphql.Field{Type: graphql.NewList(sourceType)},
		},
	})
	stateEnvelopeType := graphql.NewObject(graphql.ObjectConfig{
		Name: "StateEnvelope",
		Fields: graphql.Fields{
			"modelVersion": &graphql.Field{Type: graphql.String},
			"state":        &graphql.Field{Type: effectiveStateType},
			"provenance":   &graphql.Field{Type: stateProvenanceType},
		},
	})
	stateType := graphql.NewObject(graphql.ObjectConfig{
		Name: "State",
		Fields: graphql.Fields{
			"envelope": &graphql.Field{
				Type: stateEnvelopeType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					gctx := p.Source.(Context)
					return stateEnvelopeView(store.NewState(gctx.State, gctx.Types).StateEnvelope()), nil
				},
			},
		},
	})
	getResultType := graphql.NewObject(graphql.ObjectConfig{
		Name: "GetResult",
		Fields: graphql.Fields{
			"key":         &graphql.Field{Type: graphql.String},
			"field":       &graphql.Field{Type: fieldRefType},
			"value":       &graphql.Field{Type: graphql.String},
			"visibility":  &graphql.Field{Type: graphql.String},
			"exposure":    &graphql.Field{Type: graphql.String},
			"source":      &graphql.Field{Type: sourceType},
			"diagnostics": &graphql.Field{Type: graphql.NewList(diagnosticType)},
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
			"visibility": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return string(item.Visibility), nil
				},
			},
			"exposure": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return string(item.Exposure), nil
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
						return store.NewState(gctx.State, gctx.Types).Dotenv(store.DotenvPolicy{Insecure: insecure})
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
				"get": &graphql.Field{
					Type: getResultType,
					Args: graphql.FieldConfigArgument{
						"key":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
						"reveal": &graphql.ArgumentConfig{Type: graphql.Boolean, DefaultValue: false},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						key := p.Args["key"].(string)
						reveal, _ := p.Args["reveal"].(bool)
						result, ok, err := store.NewState(gctx.State, gctx.Types).Get(key, store.GetPolicy{Reveal: reveal})
						if err != nil || !ok {
							return nil, err
						}
						return getResultView(result), nil
					},
				},
				"sensitiveKeys": &graphql.Field{
					Type: graphql.NewList(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						return store.NewState(gctx.State, gctx.Types).SensitiveKeys()
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
				"update": &graphql.Field{
					Type: environmentType,
					Args: graphql.FieldConfigArgument{
						"dotenv": &graphql.ArgumentConfig{Type: dotenvInput},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						input := decodeDotenvInput(p.Args["dotenv"])
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.UpdateOperation{Source: input.DotenvSource, Dotenv: input.Dotenv})
						if err != nil {
							return nil, err
						}
						return Context{State: state, Types: gctx.Types}, nil
					},
				},
				"delete": &graphql.Field{
					Type: environmentType,
					Args: graphql.FieldConfigArgument{
						"keys": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.DeleteOperation{Keys: decodeStringList(p.Args["keys"])})
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
				"state": &graphql.Field{
					Type: stateType,
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
	if envelopeRaw, ok := raw["envelope"].(map[string]interface{}); ok {
		envelope := store.StateEnvelope{
			ModelVersion: stringValue(envelopeRaw["modelVersion"]),
			State:        decodeEffectiveStateInput(envelopeRaw["state"]),
		}
		input.Envelope = &envelope
	}
	dotenvInput := decodeDotenvInput(raw["dotenv"])
	input.DotenvSource = dotenvInput.DotenvSource
	input.Dotenv = dotenvInput.Dotenv
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

func decodeDotenvInput(raw interface{}) store.LoadInput {
	var input store.LoadInput
	dotenvRaw, ok := raw.(map[string]interface{})
	if !ok {
		return input
	}
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
	return input
}

func decodeEffectiveStateInput(raw interface{}) model.EffectiveState {
	state := model.NewEffectiveState()
	row, ok := raw.(map[string]interface{})
	if !ok {
		return state
	}
	for _, item := range decodeList(row["values"]) {
		valueRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		value := model.Value{
			FieldRef:    decodeFieldRef(valueRaw["field"]),
			Original:    stringValue(valueRaw["original"]),
			Resolved:    stringValue(valueRaw["resolved"]),
			Visibility:  model.Visibility(stringValue(valueRaw["visibility"])),
			Sensitivity: model.Sensitivity(stringValue(valueRaw["sensitivity"])),
			Exposure:    model.Exposure(stringValue(valueRaw["exposure"])),
			Origin:      decodeSource(valueRaw["origin"]),
			Source:      decodeSource(valueRaw["source"]),
		}
		state.Values[value.FieldRef] = value
	}
	for _, item := range decodeList(row["bindings"]) {
		bindingRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		state.Bindings = append(state.Bindings, model.Binding{
			ID:           stringValue(bindingRaw["id"]),
			FieldRef:     decodeFieldRef(bindingRaw["field"]),
			ProjectionID: model.ProjectionID(stringValue(bindingRaw["projection"])),
			Key:          model.ProjectionKey(stringValue(bindingRaw["key"])),
			Description:  stringValue(bindingRaw["description"]),
			Source:       decodeSource(bindingRaw["source"]),
			Origin:       decodeSource(bindingRaw["origin"]),
			Confidence:   model.BindingConfidence(stringValue(bindingRaw["confidence"])),
			Explicit:     boolValue(bindingRaw["explicit"]),
			PreserveKey:  boolValue(bindingRaw["preserveKey"]),
		})
	}
	for _, item := range decodeList(row["diagnostics"]) {
		diagnosticRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		state.Diagnostics = append(state.Diagnostics, model.Diagnostic{
			Severity: model.DiagnosticSeverity(stringValue(diagnosticRaw["severity"])),
			Code:     stringValue(diagnosticRaw["code"]),
			Message:  stringValue(diagnosticRaw["message"]),
			Key:      stringValue(diagnosticRaw["key"]),
			FieldRef: decodeFieldRef(diagnosticRaw["field"]),
		})
	}
	return state
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

func decodeStringList(raw interface{}) []string {
	var result []string
	for _, item := range decodeList(raw) {
		result = append(result, stringValue(item))
	}
	return result
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

func stateEnvelopeView(envelope store.StateEnvelope) map[string]interface{} {
	return map[string]interface{}{
		"modelVersion": envelope.ModelVersion,
		"state":        effectiveStateView(envelope.State),
		"provenance": map[string]interface{}{
			"sources": envelope.Provenance.Sources,
		},
	}
}

func effectiveStateView(state model.EffectiveState) map[string]interface{} {
	values := make([]map[string]interface{}, 0, len(state.Values))
	for ref, value := range state.Values {
		value.FieldRef = ref
		values = append(values, map[string]interface{}{
			"field":       fieldRefView(value.FieldRef),
			"original":    value.Original,
			"resolved":    value.Resolved,
			"visibility":  string(value.Visibility),
			"sensitivity": string(value.Sensitivity),
			"exposure":    string(value.Exposure),
			"origin":      sourceView(value.Origin),
			"source":      sourceView(value.Source),
		})
	}
	sort.SliceStable(values, func(i, j int) bool {
		return decodeFieldRef(values[i]["field"]).String() < decodeFieldRef(values[j]["field"]).String()
	})
	bindings := make([]map[string]interface{}, 0, len(state.Bindings))
	for _, binding := range state.Bindings {
		bindings = append(bindings, map[string]interface{}{
			"id":          binding.ID,
			"field":       fieldRefView(binding.FieldRef),
			"projection":  string(binding.ProjectionID),
			"key":         string(binding.Key),
			"description": binding.Description,
			"source":      sourceView(binding.Source),
			"origin":      sourceView(binding.Origin),
			"confidence":  string(binding.Confidence),
			"explicit":    binding.Explicit,
			"preserveKey": binding.PreserveKey,
		})
	}
	return map[string]interface{}{
		"values":      values,
		"bindings":    bindings,
		"diagnostics": sortedDiagnostics(state.Diagnostics),
	}
}

func getResultView(result store.GetResult) map[string]interface{} {
	return map[string]interface{}{
		"key":         result.Key,
		"field":       fieldRefView(result.Field),
		"value":       result.Value,
		"visibility":  string(result.Visibility),
		"exposure":    string(result.Exposure),
		"source":      sourceView(result.Source),
		"diagnostics": result.Diagnostics,
	}
}

func sourceView(source model.Source) map[string]interface{} {
	return map[string]interface{}{
		"name": source.Name,
		"kind": source.Kind,
	}
}

func fieldRefView(ref model.FieldRef) map[string]interface{} {
	return map[string]interface{}{
		"typeID":   string(ref.TypeID),
		"instance": ref.Instance,
		"field":    ref.Field,
	}
}
