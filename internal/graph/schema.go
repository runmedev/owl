package graph

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/resolver"
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
			"key":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"value":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"source": &graphql.InputObjectFieldConfig{Type: sourceInput},
		},
	})
	dotenvInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "DotenvInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"source":    &graphql.InputObjectFieldConfig{Type: sourceInput},
			"variables": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(dotenvVariableInput))},
			"timestamp": &graphql.InputObjectFieldConfig{Type: graphql.String},
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
			"order":       &graphql.InputObjectFieldConfig{Type: graphql.Int},
			"sensitivity": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"exposure":    &graphql.InputObjectFieldConfig{Type: graphql.String},
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
			"details":  &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
			"key":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"field":    &graphql.InputObjectFieldConfig{Type: fieldRefInput},
			"owner":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	resolverAttemptInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ResolverAttemptInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"id":            &graphql.InputObjectFieldConfig{Type: graphql.String},
			"resolverID":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"field":         &graphql.InputObjectFieldConfig{Type: fieldRefInput},
			"projectionKey": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"outcome":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"message":       &graphql.InputObjectFieldConfig{Type: graphql.String},
			"source":        &graphql.InputObjectFieldConfig{Type: sourceInput},
			"startedAt":     &graphql.InputObjectFieldConfig{Type: graphql.String},
			"finishedAt":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"diagnostics":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(diagnosticInput))},
		},
	})
	proposedValueInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ProposedValueInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"value":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"source":      &graphql.InputObjectFieldConfig{Type: sourceInput},
			"sensitivity": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"exposure":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	resolverProposalInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ResolverProposalInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"needID":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"attemptID":     &graphql.InputObjectFieldConfig{Type: graphql.String},
			"resolverID":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"field":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(fieldRefInput)},
			"projectionKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"value":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(proposedValueInput)},
		},
	})
	unresolvedNeedInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UnresolvedNeedInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"id":                 &graphql.InputObjectFieldConfig{Type: graphql.String},
			"field":              &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(fieldRefInput)},
			"projectionKey":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"required":           &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"blocking":           &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"reason":             &graphql.InputObjectFieldConfig{Type: graphql.String},
			"description":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"sensitivity":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"exposure":           &graphql.InputObjectFieldConfig{Type: graphql.String},
			"source":             &graphql.InputObjectFieldConfig{Type: sourceInput},
			"origin":             &graphql.InputObjectFieldConfig{Type: sourceInput},
			"resolverAttemptIDs": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		},
	})
	unresolvedFrontierInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UnresolvedFrontierInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"needs": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(unresolvedNeedInput))},
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
			"createdAt":   &graphql.InputObjectFieldConfig{Type: graphql.String},
			"updatedAt":   &graphql.InputObjectFieldConfig{Type: graphql.String},
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
			"order":       &graphql.InputObjectFieldConfig{Type: graphql.Int},
			"preserveKey": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"required":    &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		},
	})
	effectiveStateInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "EffectiveStateInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"values":             &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(stateValueInput))},
			"bindings":           &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(stateBindingInput))},
			"resolverAttempts":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(resolverAttemptInput))},
			"unresolvedFrontier": &graphql.InputObjectFieldConfig{Type: unresolvedFrontierInput},
			"diagnostics":        &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(diagnosticInput))},
		},
	})
	operationMetadataInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "OperationMetadataInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"id":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"kind":       &graphql.InputObjectFieldConfig{Type: graphql.String},
			"timestamp":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"actor":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"source":     &graphql.InputObjectFieldConfig{Type: sourceInput},
			"projection": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	stateProvenanceInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StateProvenanceInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"sources":    &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(sourceInput))},
			"operations": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(operationMetadataInput))},
		},
	})
	stateEnvelopeInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StateEnvelopeInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"modelVersion": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"state":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(effectiveStateInput)},
			"provenance":   &graphql.InputObjectFieldConfig{Type: stateProvenanceInput},
		},
	})
	loadInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "LoadInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"dotenv":    &graphql.InputObjectFieldConfig{Type: dotenvInput},
			"contracts": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(envContractInput))},
			"envelope":  &graphql.InputObjectFieldConfig{Type: stateEnvelopeInput},
			"timestamp": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})

	diagnosticType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Diagnostic",
		Fields: graphql.Fields{
			"severity": &graphql.Field{Type: graphql.String},
			"code":     &graphql.Field{Type: graphql.String},
			"message":  &graphql.Field{Type: graphql.String},
			"details": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					switch diagnostic := p.Source.(type) {
					case model.Diagnostic:
						return diagnostic.Details, nil
					case map[string]interface{}:
						return decodeStringList(diagnostic["details"]), nil
					}
					return nil, nil
				},
			},
			"key":   &graphql.Field{Type: graphql.String},
			"owner": &graphql.Field{Type: graphql.String},
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
			"createdAt":   &graphql.Field{Type: graphql.String},
			"updatedAt":   &graphql.Field{Type: graphql.String},
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
			"order":       &graphql.Field{Type: graphql.Int},
			"preserveKey": &graphql.Field{Type: graphql.Boolean},
			"required":    &graphql.Field{Type: graphql.Boolean},
		},
	})
	resolverAttemptType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ResolverAttempt",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.String},
			"resolverID":    &graphql.Field{Type: graphql.String},
			"field":         &graphql.Field{Type: fieldRefType},
			"projectionKey": &graphql.Field{Type: graphql.String},
			"outcome":       &graphql.Field{Type: graphql.String},
			"message":       &graphql.Field{Type: graphql.String},
			"source":        &graphql.Field{Type: sourceType},
			"startedAt":     &graphql.Field{Type: graphql.String},
			"finishedAt":    &graphql.Field{Type: graphql.String},
			"diagnostics":   &graphql.Field{Type: graphql.NewList(diagnosticType)},
		},
	})
	unresolvedNeedType := graphql.NewObject(graphql.ObjectConfig{
		Name: "UnresolvedNeed",
		Fields: graphql.Fields{
			"id":                 &graphql.Field{Type: graphql.String},
			"field":              &graphql.Field{Type: fieldRefType},
			"projectionKey":      &graphql.Field{Type: graphql.String},
			"required":           &graphql.Field{Type: graphql.Boolean},
			"blocking":           &graphql.Field{Type: graphql.Boolean},
			"reason":             &graphql.Field{Type: graphql.String},
			"description":        &graphql.Field{Type: graphql.String},
			"sensitivity":        &graphql.Field{Type: graphql.String},
			"exposure":           &graphql.Field{Type: graphql.String},
			"source":             &graphql.Field{Type: sourceType},
			"origin":             &graphql.Field{Type: sourceType},
			"resolverAttemptIDs": &graphql.Field{Type: graphql.NewList(graphql.String)},
		},
	})
	unresolvedFrontierType := graphql.NewObject(graphql.ObjectConfig{
		Name: "UnresolvedFrontier",
		Fields: graphql.Fields{
			"needs": &graphql.Field{Type: graphql.NewList(unresolvedNeedType)},
		},
	})
	effectiveStateType := graphql.NewObject(graphql.ObjectConfig{
		Name: "EffectiveState",
		Fields: graphql.Fields{
			"values":             &graphql.Field{Type: graphql.NewList(stateValueType)},
			"bindings":           &graphql.Field{Type: graphql.NewList(stateBindingType)},
			"resolverAttempts":   &graphql.Field{Type: graphql.NewList(resolverAttemptType)},
			"unresolvedFrontier": &graphql.Field{Type: unresolvedFrontierType},
			"diagnostics":        &graphql.Field{Type: graphql.NewList(diagnosticType)},
		},
	})
	operationMetadataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "OperationMetadata",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.String},
			"kind":       &graphql.Field{Type: graphql.String},
			"timestamp":  &graphql.Field{Type: graphql.String},
			"actor":      &graphql.Field{Type: graphql.String},
			"source":     &graphql.Field{Type: sourceType},
			"projection": &graphql.Field{Type: graphql.String},
		},
	})
	stateProvenanceType := graphql.NewObject(graphql.ObjectConfig{
		Name: "StateProvenance",
		Fields: graphql.Fields{
			"sources":    &graphql.Field{Type: graphql.NewList(sourceType)},
			"operations": &graphql.Field{Type: graphql.NewList(operationMetadataType)},
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
			"sourceRef": &graphql.Field{
				Type: sourceType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Source, nil
				},
			},
			"origin": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Origin.Name, nil
				},
			},
			"originRef": &graphql.Field{
				Type: sourceType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return item.Origin, nil
				},
			},
			"explicit": &graphql.Field{Type: graphql.Boolean},
			"confidence": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return string(item.Confidence), nil
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
			"updatedAt": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					item := p.Source.(store.SnapshotItem)
					return timeString(item.UpdatedAt), nil
				},
			},
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
						state, err := s.Apply(contextFromParams(p), store.LoadOperation{Input: input, Timestamp: input.Timestamp})
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
						state, err := s.Apply(contextFromParams(p), store.UpdateOperation{Source: input.DotenvSource, Dotenv: input.Dotenv, Timestamp: input.Timestamp})
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
				"recordResolverAttempt": &graphql.Field{
					Type: environmentType,
					Args: graphql.FieldConfigArgument{
						"attempt": &graphql.ArgumentConfig{Type: graphql.NewNonNull(resolverAttemptInput)},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						attempt := decodeResolverAttemptInput(p.Args["attempt"])
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.RecordResolverAttemptOperation{Attempt: attempt})
						if err != nil {
							return nil, err
						}
						return Context{State: state, Types: gctx.Types}, nil
					},
				},
				"applyResolverProposal": &graphql.Field{
					Type: environmentType,
					Args: graphql.FieldConfigArgument{
						"proposal":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(resolverProposalInput)},
						"timestamp": &graphql.ArgumentConfig{Type: graphql.String},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						gctx := p.Source.(Context)
						proposal := decodeResolverProposalInput(p.Args["proposal"])
						s := store.NewState(gctx.State, gctx.Types)
						state, err := s.Apply(contextFromParams(p), store.ApplyResolverProposalOperation{
							Proposal:  proposal,
							Timestamp: timeValue(p.Args["timestamp"]),
						})
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
						state, err := s.Apply(contextFromParams(p), store.IntegrityOperation{Types: gctx.Types})
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
			Provenance:   decodeStateProvenanceInput(envelopeRaw["provenance"]),
		}
		input.Envelope = &envelope
	}
	input.Timestamp = timeValue(raw["timestamp"])
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
				Order:       uintValue(bindingRaw["order"]),
				Sensitivity: model.Sensitivity(stringValue(bindingRaw["sensitivity"])),
				Exposure:    model.Exposure(stringValue(bindingRaw["exposure"])),
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
	input.Timestamp = timeValue(dotenvRaw["timestamp"])
	for _, item := range decodeList(dotenvRaw["variables"]) {
		variable, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		input.Dotenv = append(input.Dotenv, store.DotenvVariable{
			Key:    stringValue(variable["key"]),
			Value:  stringValue(variable["value"]),
			Source: decodeSource(variable["source"]),
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
			CreatedAt:   timeValue(valueRaw["createdAt"]),
			UpdatedAt:   timeValue(valueRaw["updatedAt"]),
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
			Order:        uintValue(bindingRaw["order"]),
			PreserveKey:  boolValue(bindingRaw["preserveKey"]),
			Required:     boolValue(bindingRaw["required"]),
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
			Details:  decodeStringList(diagnosticRaw["details"]),
			Key:      stringValue(diagnosticRaw["key"]),
			FieldRef: decodeFieldRef(diagnosticRaw["field"]),
			Owner:    model.DiagnosticOwner(stringValue(diagnosticRaw["owner"])),
		})
	}
	for _, item := range decodeList(row["resolverAttempts"]) {
		state.ResolverAttempts = append(state.ResolverAttempts, decodeResolverAttemptInput(item))
	}
	state.UnresolvedFrontier = decodeUnresolvedFrontierInput(row["unresolvedFrontier"])
	return state
}

func decodeUnresolvedFrontierInput(raw interface{}) model.UnresolvedFrontier {
	var frontier model.UnresolvedFrontier
	row, ok := raw.(map[string]interface{})
	if !ok {
		return frontier
	}
	for _, item := range decodeList(row["needs"]) {
		needRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		frontier.Needs = append(frontier.Needs, model.UnresolvedNeed{
			ID:                 model.UnresolvedNeedID(stringValue(needRaw["id"])),
			FieldRef:           decodeFieldRef(needRaw["field"]),
			ProjectionKey:      model.ProjectionKey(stringValue(needRaw["projectionKey"])),
			Required:           boolValue(needRaw["required"]),
			Blocking:           boolValue(needRaw["blocking"]),
			Reason:             model.UnresolvedReason(stringValue(needRaw["reason"])),
			Description:        stringValue(needRaw["description"]),
			Sensitivity:        model.Sensitivity(stringValue(needRaw["sensitivity"])),
			Exposure:           model.Exposure(stringValue(needRaw["exposure"])),
			Source:             decodeSource(needRaw["source"]),
			Origin:             decodeSource(needRaw["origin"]),
			ResolverAttemptIDs: decodeResolverAttemptIDs(needRaw["resolverAttemptIDs"]),
		})
	}
	return frontier
}

func decodeResolverAttemptIDs(raw interface{}) []model.ResolverAttemptID {
	var result []model.ResolverAttemptID
	for _, id := range decodeStringList(raw) {
		result = append(result, model.ResolverAttemptID(id))
	}
	return result
}

func decodeResolverAttemptInput(raw interface{}) model.ResolverAttempt {
	attemptRaw, ok := raw.(map[string]interface{})
	if !ok {
		return model.ResolverAttempt{}
	}
	return model.ResolverAttempt{
		ID:            model.ResolverAttemptID(stringValue(attemptRaw["id"])),
		ResolverID:    model.ResolverID(stringValue(attemptRaw["resolverID"])),
		FieldRef:      decodeFieldRef(attemptRaw["field"]),
		ProjectionKey: model.ProjectionKey(stringValue(attemptRaw["projectionKey"])),
		Outcome:       model.ResolverAttemptOutcome(stringValue(attemptRaw["outcome"])),
		Message:       stringValue(attemptRaw["message"]),
		Source:        decodeSource(attemptRaw["source"]),
		StartedAt:     timeValue(attemptRaw["startedAt"]),
		FinishedAt:    timeValue(attemptRaw["finishedAt"]),
		Diagnostics:   decodeDiagnosticsInput(attemptRaw["diagnostics"]),
	}
}

func decodeDiagnosticsInput(raw interface{}) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	for _, item := range decodeList(raw) {
		diagnosticRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, model.Diagnostic{
			Severity: model.DiagnosticSeverity(stringValue(diagnosticRaw["severity"])),
			Code:     stringValue(diagnosticRaw["code"]),
			Message:  stringValue(diagnosticRaw["message"]),
			Details:  decodeStringList(diagnosticRaw["details"]),
			Key:      stringValue(diagnosticRaw["key"]),
			FieldRef: decodeFieldRef(diagnosticRaw["field"]),
			Owner:    model.DiagnosticOwner(stringValue(diagnosticRaw["owner"])),
		})
	}
	return diagnostics
}

func decodeStateProvenanceInput(raw interface{}) store.StateProvenance {
	var provenance store.StateProvenance
	row, ok := raw.(map[string]interface{})
	if !ok {
		return provenance
	}
	for _, item := range decodeList(row["sources"]) {
		provenance.Sources = append(provenance.Sources, decodeSource(item))
	}
	for _, item := range decodeList(row["operations"]) {
		operationRaw, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		provenance.Operations = append(provenance.Operations, model.OperationMetadata{
			ID:           model.OperationID(stringValue(operationRaw["id"])),
			Kind:         model.OperationKind(stringValue(operationRaw["kind"])),
			Timestamp:    timeValue(operationRaw["timestamp"]),
			Actor:        stringValue(operationRaw["actor"]),
			Source:       decodeSource(operationRaw["source"]),
			ProjectionID: model.ProjectionID(stringValue(operationRaw["projection"])),
		})
	}
	return provenance
}

func decodeSource(raw interface{}) model.Source {
	source, ok := raw.(map[string]interface{})
	if !ok {
		return model.Source{}
	}
	return model.Source{Name: stringValue(source["name"]), Kind: stringValue(source["kind"])}
}

func decodeFieldRef(raw interface{}) model.FieldRef {
	if ref, ok := raw.(string); ok {
		return decodeFieldRefString(ref)
	}
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

func decodeFieldRefString(raw string) model.FieldRef {
	typeRef, field, ok := strings.Cut(raw, ".")
	if !ok {
		return model.FieldRef{}
	}
	instance := ""
	if before, after, ok := strings.Cut(typeRef, "("); ok {
		typeRef = before
		instance = strings.Trim(after, ")\"")
	}
	typeID, err := model.ParseTypeID(typeRef)
	if err != nil {
		return model.FieldRef{}
	}
	return model.FieldRef{TypeID: typeID, Instance: instance, Field: field}
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

func uintValue(raw interface{}) uint {
	switch value := raw.(type) {
	case int:
		if value > 0 {
			return uint(value)
		}
	case int32:
		if value > 0 {
			return uint(value)
		}
	case int64:
		if value > 0 {
			return uint(value)
		}
	case float64:
		if value > 0 {
			return uint(value)
		}
	}
	return 0
}

func timeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func timeValue(raw interface{}) time.Time {
	value := stringValue(raw)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func stateEnvelopeView(envelope store.StateEnvelope) map[string]interface{} {
	return map[string]interface{}{
		"modelVersion": envelope.ModelVersion,
		"state":        effectiveStateView(envelope.State),
		"provenance": map[string]interface{}{
			"sources":    envelope.Provenance.Sources,
			"operations": operationMetadataViews(envelope.Provenance.Operations),
		},
	}
}

func decodeResolverProposalInput(raw interface{}) resolver.Proposal {
	proposalRaw, ok := raw.(map[string]interface{})
	if !ok {
		return resolver.Proposal{}
	}
	return resolver.Proposal{
		NeedID:        model.UnresolvedNeedID(stringValue(proposalRaw["needID"])),
		AttemptID:     model.ResolverAttemptID(stringValue(proposalRaw["attemptID"])),
		ResolverID:    model.ResolverID(stringValue(proposalRaw["resolverID"])),
		FieldRef:      decodeFieldRef(proposalRaw["field"]),
		ProjectionKey: model.ProjectionKey(stringValue(proposalRaw["projectionKey"])),
		Value:         decodeProposedValueInput(proposalRaw["value"]),
	}
}

func decodeProposedValueInput(raw interface{}) resolver.ProposedValue {
	valueRaw, ok := raw.(map[string]interface{})
	if !ok {
		return resolver.ProposedValue{}
	}
	return resolver.ProposedValue{
		Value:       stringValue(valueRaw["value"]),
		Source:      decodeSource(valueRaw["source"]),
		Sensitivity: model.Sensitivity(stringValue(valueRaw["sensitivity"])),
		Exposure:    model.Exposure(stringValue(valueRaw["exposure"])),
	}
}

func operationMetadataViews(operations []model.OperationMetadata) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(operations))
	for _, operation := range operations {
		items = append(items, map[string]interface{}{
			"id":         string(operation.ID),
			"kind":       string(operation.Kind),
			"timestamp":  timeString(operation.Timestamp),
			"actor":      operation.Actor,
			"source":     sourceView(operation.Source),
			"projection": string(operation.ProjectionID),
		})
	}
	return items
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
			"createdAt":   timeString(value.CreatedAt),
			"updatedAt":   timeString(value.UpdatedAt),
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
			"order":       int(binding.Order),
			"preserveKey": binding.PreserveKey,
			"required":    binding.Required,
		})
	}
	return map[string]interface{}{
		"values":             values,
		"bindings":           bindings,
		"resolverAttempts":   resolverAttemptViews(state.ResolverAttempts),
		"unresolvedFrontier": unresolvedFrontierView(state.UnresolvedFrontier),
		"diagnostics":        sortedDiagnostics(state.Diagnostics),
	}
}

func resolverAttemptViews(attempts []model.ResolverAttempt) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, resolverAttemptView(attempt))
	}
	return items
}

func resolverAttemptView(attempt model.ResolverAttempt) map[string]interface{} {
	return map[string]interface{}{
		"id":            string(attempt.ID),
		"resolverID":    string(attempt.ResolverID),
		"field":         fieldRefView(attempt.FieldRef),
		"projectionKey": string(attempt.ProjectionKey),
		"outcome":       string(attempt.Outcome),
		"message":       attempt.Message,
		"source":        sourceView(attempt.Source),
		"startedAt":     timeString(attempt.StartedAt),
		"finishedAt":    timeString(attempt.FinishedAt),
		"diagnostics":   sortedDiagnostics(attempt.Diagnostics),
	}
}

func unresolvedFrontierView(frontier model.UnresolvedFrontier) map[string]interface{} {
	needs := make([]map[string]interface{}, 0, len(frontier.Needs))
	for _, need := range frontier.Needs {
		ids := make([]string, 0, len(need.ResolverAttemptIDs))
		for _, id := range need.ResolverAttemptIDs {
			ids = append(ids, string(id))
		}
		needs = append(needs, map[string]interface{}{
			"id":                 string(need.ID),
			"field":              fieldRefView(need.FieldRef),
			"projectionKey":      string(need.ProjectionKey),
			"required":           need.Required,
			"blocking":           need.Blocking,
			"reason":             string(need.Reason),
			"description":        need.Description,
			"sensitivity":        string(need.Sensitivity),
			"exposure":           string(need.Exposure),
			"source":             sourceView(need.Source),
			"origin":             sourceView(need.Origin),
			"resolverAttemptIDs": ids,
		})
	}
	return map[string]interface{}{"needs": needs}
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
