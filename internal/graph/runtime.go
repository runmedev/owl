package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/graphql-go/graphql"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/store"
)

type Runtime struct {
	schema graphql.Schema
	types  registry.TypeProvider
}

type Context struct {
	State model.EffectiveState
	Types registry.TypeProvider
}

type LoadInput = store.LoadInput

type SnapshotPolicy = store.SnapshotPolicy

type DotenvPolicy = store.DotenvPolicy

type GetPolicy = store.GetPolicy

type SnapshotItem = store.SnapshotItem

type GetResult = store.GetResult

type CheckResult = store.CheckResult

type StateEnvelope = store.StateEnvelope

func NewRuntime(types registry.TypeProvider) (*Runtime, error) {
	if types == nil {
		types = registry.NewBuiltInRegistry()
	}
	r := &Runtime{types: types}
	schema, err := r.newSchema()
	if err != nil {
		return nil, err
	}
	r.schema = schema
	return r, nil
}

func (r *Runtime) Snapshot(ctx context.Context, input LoadInput, policy SnapshotPolicy) ([]SnapshotItem, error) {
	result, err := r.do(ctx, snapshotQuery, map[string]interface{}{
		"input":  marshalInput(input),
		"reveal": policy.Reveal,
	})
	if err != nil {
		return nil, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "snapshot")
	if err != nil {
		return nil, err
	}
	return decodeSnapshot(raw), nil
}

func (r *Runtime) Dotenv(ctx context.Context, input LoadInput, policy DotenvPolicy) ([]string, error) {
	result, err := r.do(ctx, dotenvQuery, map[string]interface{}{
		"input":    marshalInput(input),
		"insecure": policy.Insecure,
	})
	if err != nil {
		return nil, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "dotenv")
	if err != nil {
		return nil, err
	}
	var envs []string
	if err := remarshal(raw, &envs); err != nil {
		return nil, err
	}
	return envs, nil
}

func (r *Runtime) Get(ctx context.Context, input LoadInput, key string, policy GetPolicy) (GetResult, bool, error) {
	result, err := r.do(ctx, getQuery, map[string]interface{}{
		"input":  marshalInput(input),
		"key":    key,
		"reveal": policy.Reveal,
	})
	if err != nil {
		return GetResult{}, false, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "get")
	if err != nil {
		return GetResult{}, false, err
	}
	if raw == nil {
		return GetResult{}, false, nil
	}
	return decodeGet(raw), true, nil
}

func (r *Runtime) SensitiveKeys(ctx context.Context, input LoadInput) ([]string, error) {
	result, err := r.do(ctx, sensitiveKeysQuery, map[string]interface{}{
		"input": marshalInput(input),
	})
	if err != nil {
		return nil, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "sensitiveKeys")
	if err != nil {
		return nil, err
	}
	return decodeStringList(raw), nil
}

func (r *Runtime) StateEnvelope(ctx context.Context, input LoadInput) (StateEnvelope, error) {
	return r.StateEnvelopeAfter(ctx, input, store.LoadInput{}, nil)
}

func (r *Runtime) StateEnvelopeAfter(ctx context.Context, input LoadInput, patch store.LoadInput, deleted []string) (StateEnvelope, error) {
	result, err := r.do(ctx, stateEnvelopeQuery, map[string]interface{}{
		"input":   marshalInput(input),
		"updates": marshalDotenvInput(patch),
		"deleted": deleted,
	})
	if err != nil {
		return StateEnvelope{}, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "update", "delete", "normalize", "validate", "state", "envelope")
	if err != nil {
		return StateEnvelope{}, err
	}
	return decodeEnvelope(raw)
}

func (r *Runtime) Check(ctx context.Context, input LoadInput) (CheckResult, error) {
	result, err := r.do(ctx, checkQuery, map[string]interface{}{
		"input": marshalInput(input),
	})
	if err != nil {
		return CheckResult{}, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "check")
	if err != nil {
		return CheckResult{}, err
	}
	return decodeCheck(raw), nil
}

func (r *Runtime) SchemaJSON(ctx context.Context) (string, error) {
	result, err := r.do(ctx, introspectionQuery, nil)
	if err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(result.Data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (r *Runtime) do(ctx context.Context, query string, vars map[string]interface{}) (*graphql.Result, error) {
	result := graphql.Do(graphql.Params{
		Schema:         r.schema,
		RequestString:  query,
		VariableValues: vars,
		Context:        ctx,
	})
	if result.HasErrors() {
		return nil, fmt.Errorf("graphql errors %s", result.Errors)
	}
	return result, nil
}

func marshalInput(input LoadInput) map[string]interface{} {
	variables := make([]map[string]interface{}, 0, len(input.Dotenv))
	for _, variable := range input.Dotenv {
		variables = append(variables, map[string]interface{}{
			"key":   variable.Key,
			"value": variable.Value,
		})
	}
	contracts := make([]map[string]interface{}, 0, len(input.Contracts))
	for _, contract := range input.Contracts {
		bindings := make([]map[string]interface{}, 0, len(contract.Bindings))
		for _, binding := range contract.Bindings {
			bindingInput := map[string]interface{}{
				"field": map[string]interface{}{
					"typeID":   string(binding.FieldRef.TypeID),
					"instance": binding.FieldRef.Instance,
					"field":    binding.FieldRef.Field,
				},
				"key":         binding.Key,
				"projection":  string(binding.Projection),
				"required":    binding.Required,
				"description": binding.Description,
			}
			if source := marshalSource(binding.Source); source != nil {
				bindingInput["source"] = source
			}
			bindings = append(bindings, bindingInput)
		}
		contractInput := map[string]interface{}{
			"projection": string(contract.Projection),
			"bindings":   bindings,
		}
		if source := marshalSource(contract.Source); source != nil {
			contractInput["source"] = source
		}
		contracts = append(contracts, contractInput)
	}
	dotenv := map[string]interface{}{
		"variables": variables,
	}
	if source := marshalSource(input.DotenvSource); source != nil {
		dotenv["source"] = source
	}
	result := map[string]interface{}{
		"dotenv":    dotenv,
		"contracts": contracts,
	}
	if input.Envelope != nil {
		result["envelope"] = marshalEnvelope(*input.Envelope)
	}
	return result
}

func marshalDotenvInput(input LoadInput) map[string]interface{} {
	if len(input.Dotenv) == 0 {
		return nil
	}
	variables := make([]map[string]interface{}, 0, len(input.Dotenv))
	for _, variable := range input.Dotenv {
		variables = append(variables, map[string]interface{}{
			"key":   variable.Key,
			"value": variable.Value,
		})
	}
	dotenv := map[string]interface{}{"variables": variables}
	if source := marshalSource(input.DotenvSource); source != nil {
		dotenv["source"] = source
	}
	return dotenv
}

func marshalEnvelope(envelope StateEnvelope) map[string]interface{} {
	return map[string]interface{}{
		"modelVersion": envelope.ModelVersion,
		"state":        marshalEffectiveState(envelope.State),
	}
}

func marshalEffectiveState(state model.EffectiveState) map[string]interface{} {
	values := make([]map[string]interface{}, 0, len(state.Values))
	for ref, value := range state.Values {
		value.FieldRef = ref
		values = append(values, map[string]interface{}{
			"field":       marshalFieldRef(value.FieldRef),
			"original":    value.Original,
			"resolved":    value.Resolved,
			"visibility":  string(value.Visibility),
			"sensitivity": string(value.Sensitivity),
			"exposure":    string(value.Exposure),
			"origin":      marshalSource(value.Origin),
			"source":      marshalSource(value.Source),
		})
	}
	bindings := make([]map[string]interface{}, 0, len(state.Bindings))
	for _, binding := range state.Bindings {
		bindings = append(bindings, map[string]interface{}{
			"id":          binding.ID,
			"field":       marshalFieldRef(binding.FieldRef),
			"projection":  string(binding.ProjectionID),
			"key":         string(binding.Key),
			"description": binding.Description,
			"source":      marshalSource(binding.Source),
			"origin":      marshalSource(binding.Origin),
			"confidence":  string(binding.Confidence),
			"explicit":    binding.Explicit,
			"preserveKey": binding.PreserveKey,
		})
	}
	diagnostics := make([]map[string]interface{}, 0, len(state.Diagnostics))
	for _, diagnostic := range state.Diagnostics {
		diagnostics = append(diagnostics, map[string]interface{}{
			"severity": string(diagnostic.Severity),
			"code":     diagnostic.Code,
			"message":  diagnostic.Message,
			"key":      diagnostic.Key,
			"field":    marshalFieldRef(diagnostic.FieldRef),
		})
	}
	return map[string]interface{}{
		"values":      values,
		"bindings":    bindings,
		"diagnostics": diagnostics,
	}
}

func marshalFieldRef(ref model.FieldRef) map[string]interface{} {
	return map[string]interface{}{
		"typeID":   string(ref.TypeID),
		"instance": ref.Instance,
		"field":    ref.Field,
	}
}

func marshalSource(source model.Source) map[string]interface{} {
	if source.Name == "" && source.Kind == "" {
		return nil
	}
	return map[string]interface{}{
		"name": source.Name,
		"kind": source.Kind,
	}
}

func extractPath(data interface{}, path ...string) (interface{}, error) {
	current := data
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("graphql result path %q is not an object", key)
		}
		current, ok = m[key]
		if !ok {
			return nil, fmt.Errorf("graphql result path %q missing", key)
		}
	}
	return current, nil
}

func remarshal(src interface{}, dst interface{}) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func sortedDiagnostics(diagnostics []model.Diagnostic) []model.Diagnostic {
	out := append([]model.Diagnostic{}, diagnostics...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Key < out[j].Key
	})
	return out
}

const snapshotQuery = `
query OwlSnapshot($input: LoadInput!, $reveal: Boolean = false) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            snapshot(reveal: $reveal) {
              name
              value
              originalValue
              type
              field
              fieldTypeID
              fieldInstance
              fieldName
              source
              origin
              visibility
              description
              diagnostics { severity code message key field }
            }
          }
        }
      }
    }
  }
}`

func decodeSnapshot(raw interface{}) []SnapshotItem {
	rows, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	items := make([]SnapshotItem, 0, len(rows))
	for _, row := range rows {
		item, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		items = append(items, SnapshotItem{
			Name:          stringValue(item["name"]),
			Value:         stringValue(item["value"]),
			OriginalValue: stringValue(item["originalValue"]),
			Type:          model.TypeID(stringValue(item["type"])),
			Field: model.FieldRef{
				TypeID:   model.TypeID(stringValue(item["fieldTypeID"])),
				Instance: stringValue(item["fieldInstance"]),
				Field:    stringValue(item["fieldName"]),
			},
			Source:      model.Source{Name: stringValue(item["source"])},
			Origin:      model.Source{Name: stringValue(item["origin"])},
			Visibility:  model.Visibility(stringValue(item["visibility"])),
			Description: stringValue(item["description"]),
			Diagnostics: decodeDiagnostics(item["diagnostics"]),
		})
	}
	return items
}

func decodeCheck(raw interface{}) CheckResult {
	row, ok := raw.(map[string]interface{})
	if !ok {
		return CheckResult{}
	}
	return CheckResult{
		OK:          boolValue(row["ok"]),
		Diagnostics: decodeDiagnostics(row["diagnostics"]),
	}
}

func decodeGet(raw interface{}) GetResult {
	row, ok := raw.(map[string]interface{})
	if !ok {
		return GetResult{}
	}
	return GetResult{
		Key:         stringValue(row["key"]),
		Field:       decodeFieldRef(row["field"]),
		Value:       stringValue(row["value"]),
		Visibility:  model.Visibility(stringValue(row["visibility"])),
		Source:      decodeSource(row["source"]),
		Diagnostics: decodeDiagnostics(row["diagnostics"]),
	}
}

func decodeEnvelope(raw interface{}) (StateEnvelope, error) {
	row, ok := raw.(map[string]interface{})
	if !ok {
		return StateEnvelope{}, nil
	}
	return StateEnvelope{
		ModelVersion: stringValue(row["modelVersion"]),
		State:        decodeEffectiveState(row["state"]),
	}, nil
}

func decodeEffectiveState(raw interface{}) model.EffectiveState {
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
	state.Diagnostics = decodeDiagnostics(row["diagnostics"])
	return state
}

func decodeDiagnostics(raw interface{}) []model.Diagnostic {
	rows, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	diagnostics := make([]model.Diagnostic, 0, len(rows))
	for _, row := range rows {
		item, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		diagnostics = append(diagnostics, model.Diagnostic{
			Severity: model.DiagnosticSeverity(stringValue(item["severity"])),
			Code:     stringValue(item["code"]),
			Message:  stringValue(item["message"]),
			Key:      stringValue(item["key"]),
			FieldRef: decodeFieldRef(item["field"]),
		})
	}
	return diagnostics
}

const dotenvQuery = `
query OwlDotenv($input: LoadInput!, $insecure: Boolean = false) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            dotenv(insecure: $insecure)
          }
        }
      }
    }
  }
}`

const getQuery = `
query OwlGet($input: LoadInput!, $key: String!, $reveal: Boolean = false) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            get(key: $key, reveal: $reveal) {
              key
              field { typeID instance field }
              value
              visibility
              source { name kind }
              diagnostics { severity code message key field }
            }
          }
        }
      }
    }
  }
}`

const sensitiveKeysQuery = `
query OwlSensitiveKeys($input: LoadInput!) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            sensitiveKeys
          }
        }
      }
    }
  }
}`

const stateEnvelopeQuery = `
query OwlStateEnvelope($input: LoadInput!, $updates: DotenvInput, $deleted: [String!]) {
  Environment {
    load(input: $input) {
      update(dotenv: $updates) {
        delete(keys: $deleted) {
          normalize {
            validate {
              state {
                envelope {
                  modelVersion
                  state {
                    values {
                      field { typeID instance field }
                      original
                      resolved
                      visibility
                      sensitivity
                      exposure
                      origin { name kind }
                      source { name kind }
                    }
                    bindings {
                      id
                      field { typeID instance field }
                      projection
                      key
                      description
                      source { name kind }
                      origin { name kind }
                      confidence
                      explicit
                      preserveKey
                    }
                    diagnostics { severity code message key field }
                  }
                  provenance { sources { name kind } }
                }
              }
            }
          }
        }
      }
    }
  }
}`

const checkQuery = `
query OwlCheck($input: LoadInput!) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            check { ok diagnostics { severity code message key field } }
          }
        }
      }
    }
  }
}`

const introspectionQuery = `
query OwlSchema {
  __schema {
    queryType { name }
    types {
      kind
      name
      fields(includeDeprecated: true) {
        name
        args {
          name
          type { kind name ofType { kind name ofType { kind name } } }
          defaultValue
        }
        type { kind name ofType { kind name ofType { kind name } } }
        isDeprecated
        deprecationReason
      }
      inputFields {
        name
        type { kind name ofType { kind name ofType { kind name } } }
        defaultValue
      }
      enumValues(includeDeprecated: true) {
        name
        isDeprecated
        deprecationReason
      }
    }
  }
}`
