package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/graphql-go/graphql"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/resolver"
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

type Operation struct {
	Name      string
	Document  string
	Variables map[string]interface{}
}

type ExecuteResult struct {
	Data json.RawMessage
}

var traceGraphQLQuery func(query string, vars map[string]interface{})

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
	op := SnapshotOperation(input, policy)
	result, err := r.do(ctx, op.Document, op.Variables)
	if err != nil {
		return nil, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "snapshot")
	if err != nil {
		return nil, err
	}
	return decodeSnapshot(raw), nil
}

func SnapshotOperation(input LoadInput, policy SnapshotPolicy) Operation {
	return Operation{
		Name:     "OwlSnapshot",
		Document: snapshotQuery,
		Variables: map[string]interface{}{
			"input":  marshalInput(input),
			"reveal": policy.Reveal,
		},
	}
}

func (r *Runtime) Dotenv(ctx context.Context, input LoadInput, policy DotenvPolicy) ([]string, error) {
	op := DotenvOperation(input, policy)
	result, err := r.do(ctx, op.Document, op.Variables)
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

func DotenvOperation(input LoadInput, policy DotenvPolicy) Operation {
	return Operation{
		Name:     "OwlDotenv",
		Document: dotenvQuery,
		Variables: map[string]interface{}{
			"input":    marshalInput(input),
			"insecure": policy.Insecure,
		},
	}
}

func (r *Runtime) Get(ctx context.Context, input LoadInput, key string, policy GetPolicy) (GetResult, bool, error) {
	op := GetOperation(input, key, policy)
	result, err := r.do(ctx, op.Document, op.Variables)
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

func GetOperation(input LoadInput, key string, policy GetPolicy) Operation {
	return Operation{
		Name:     "OwlGet",
		Document: getQuery,
		Variables: map[string]interface{}{
			"input":  marshalInput(input),
			"key":    key,
			"reveal": policy.Reveal,
		},
	}
}

func (r *Runtime) SensitiveKeys(ctx context.Context, input LoadInput) ([]string, error) {
	op := SensitiveKeysOperation(input)
	result, err := r.do(ctx, op.Document, op.Variables)
	if err != nil {
		return nil, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "sensitiveKeys")
	if err != nil {
		return nil, err
	}
	return decodeStringList(raw), nil
}

func SensitiveKeysOperation(input LoadInput) Operation {
	return Operation{
		Name:     "OwlSensitiveKeys",
		Document: sensitiveKeysQuery,
		Variables: map[string]interface{}{
			"input": marshalInput(input),
		},
	}
}

func (r *Runtime) DotenvSpec(ctx context.Context, input LoadInput) (string, error) {
	op := DotenvSpecOperation(input)
	result, err := r.do(ctx, op.Document, op.Variables)
	if err != nil {
		return "", err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "dotenvSpec")
	if err != nil {
		return "", err
	}
	return stringValue(raw), nil
}

func DotenvSpecOperation(input LoadInput) Operation {
	return Operation{
		Name:     "OwlDotenvSpec",
		Document: dotenvSpecQuery,
		Variables: map[string]interface{}{
			"input": marshalInput(input),
		},
	}
}

func (r *Runtime) StateEnvelope(ctx context.Context, input LoadInput) (StateEnvelope, error) {
	return r.StateEnvelopeForOperations(ctx, []store.OperationRecord{
		{Kind: store.OperationRecordLoad, Load: input},
	})
}

func (r *Runtime) StateEnvelopeForOperations(ctx context.Context, records []store.OperationRecord) (StateEnvelope, error) {
	plan, err := planStateEnvelopeQuery(records)
	if err != nil {
		return StateEnvelope{}, err
	}
	result, err := r.do(ctx, plan.Query, plan.Vars)
	if err != nil {
		return StateEnvelope{}, err
	}
	raw, err := extractPath(result.Data, append([]string{"Environment"}, plan.Path...)...)
	if err != nil {
		return StateEnvelope{}, err
	}
	return decodeEnvelope(raw)
}

func (r *Runtime) StateEnvelopeAfter(ctx context.Context, input LoadInput, patch store.LoadInput, deleted []string) (StateEnvelope, error) {
	records := []store.OperationRecord{{Kind: store.OperationRecordLoad, Load: input}}
	if len(patch.Dotenv) > 0 {
		records = append(records, store.OperationRecord{
			Kind: store.OperationRecordUpdate,
			Update: store.UpdateOperation{
				Source: patch.DotenvSource,
				Dotenv: patch.Dotenv,
			},
		})
	}
	if len(deleted) > 0 {
		records = append(records, store.OperationRecord{
			Kind:   store.OperationRecordDelete,
			Delete: store.DeleteOperation{Keys: append([]string{}, deleted...)},
		})
	}
	return r.StateEnvelopeForOperations(ctx, records)
}

func (r *Runtime) Check(ctx context.Context, input LoadInput) (CheckResult, error) {
	op := CheckOperation(input)
	result, err := r.do(ctx, op.Document, op.Variables)
	if err != nil {
		return CheckResult{}, err
	}
	raw, err := extractPath(result.Data, "Environment", "load", "normalize", "validate", "render", "check")
	if err != nil {
		return CheckResult{}, err
	}
	return decodeCheck(raw), nil
}

func CheckOperation(input LoadInput) Operation {
	return Operation{
		Name:     "OwlCheck",
		Document: checkQuery,
		Variables: map[string]interface{}{
			"input": marshalInput(input),
		},
	}
}

func (r *Runtime) Execute(ctx context.Context, query string, vars map[string]interface{}) (ExecuteResult, error) {
	result, err := r.do(ctx, query, vars)
	if err != nil {
		return ExecuteResult{}, err
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		return ExecuteResult{}, err
	}
	return ExecuteResult{Data: raw}, nil
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
	if traceGraphQLQuery != nil {
		traceGraphQLQuery(query, vars)
	}

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
		item := map[string]interface{}{
			"key":   variable.Key,
			"value": variable.Value,
		}
		if source := marshalSource(variable.Source); source != nil {
			item["source"] = source
		}
		variables = append(variables, item)
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
				"order":       int(binding.Order),
				"sensitivity": string(binding.Sensitivity),
				"exposure":    string(binding.Exposure),
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
	if !input.Timestamp.IsZero() {
		dotenv["timestamp"] = timeString(input.Timestamp)
	}
	result := map[string]interface{}{
		"dotenv":    dotenv,
		"contracts": contracts,
	}
	if !input.Timestamp.IsZero() {
		result["timestamp"] = timeString(input.Timestamp)
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
		item := map[string]interface{}{
			"key":   variable.Key,
			"value": variable.Value,
		}
		if source := marshalSource(variable.Source); source != nil {
			item["source"] = source
		}
		variables = append(variables, item)
	}
	dotenv := map[string]interface{}{"variables": variables}
	if source := marshalSource(input.DotenvSource); source != nil {
		dotenv["source"] = source
	}
	if !input.Timestamp.IsZero() {
		dotenv["timestamp"] = timeString(input.Timestamp)
	}
	return dotenv
}

func marshalEnvelope(envelope StateEnvelope) map[string]interface{} {
	return map[string]interface{}{
		"modelVersion": envelope.ModelVersion,
		"state":        marshalEffectiveState(envelope.State),
		"provenance":   marshalStateProvenance(envelope.Provenance),
	}
}

func marshalStateProvenance(provenance store.StateProvenance) map[string]interface{} {
	sources := make([]map[string]interface{}, 0, len(provenance.Sources))
	for _, source := range provenance.Sources {
		if marshaled := marshalSource(source); marshaled != nil {
			sources = append(sources, marshaled)
		}
	}
	operations := make([]map[string]interface{}, 0, len(provenance.Operations))
	for _, operation := range provenance.Operations {
		operations = append(operations, map[string]interface{}{
			"id":         string(operation.ID),
			"kind":       string(operation.Kind),
			"timestamp":  timeString(operation.Timestamp),
			"actor":      operation.Actor,
			"source":     marshalSource(operation.Source),
			"projection": string(operation.ProjectionID),
		})
	}
	return map[string]interface{}{
		"sources":    sources,
		"operations": operations,
	}
}

func marshalEffectiveState(state model.EffectiveState) map[string]interface{} {
	values := make([]map[string]interface{}, 0, len(state.Values))
	for ref, value := range state.Values {
		value.FieldRef = ref
		item := map[string]interface{}{
			"field":       marshalFieldRef(value.FieldRef),
			"original":    value.Original,
			"resolved":    value.Resolved,
			"visibility":  string(value.Visibility),
			"sensitivity": string(value.Sensitivity),
			"exposure":    string(value.Exposure),
			"createdAt":   timeString(value.CreatedAt),
			"updatedAt":   timeString(value.UpdatedAt),
		}
		if source := marshalSource(value.Origin); source != nil {
			item["origin"] = source
		}
		if source := marshalSource(value.Source); source != nil {
			item["source"] = source
		}
		values = append(values, item)
	}
	bindings := make([]map[string]interface{}, 0, len(state.Bindings))
	for _, binding := range state.Bindings {
		item := map[string]interface{}{
			"id":          binding.ID,
			"field":       marshalFieldRef(binding.FieldRef),
			"projection":  string(binding.ProjectionID),
			"key":         string(binding.Key),
			"description": binding.Description,
			"confidence":  string(binding.Confidence),
			"explicit":    binding.Explicit,
			"order":       int(binding.Order),
			"preserveKey": binding.PreserveKey,
			"required":    binding.Required,
		}
		if source := marshalSource(binding.Source); source != nil {
			item["source"] = source
		}
		if source := marshalSource(binding.Origin); source != nil {
			item["origin"] = source
		}
		bindings = append(bindings, item)
	}
	diagnostics := make([]map[string]interface{}, 0, len(state.Diagnostics))
	for _, diagnostic := range state.Diagnostics {
		diagnostics = append(diagnostics, map[string]interface{}{
			"severity": string(diagnostic.Severity),
			"code":     diagnostic.Code,
			"message":  diagnostic.Message,
			"details":  append([]string{}, diagnostic.Details...),
			"key":      diagnostic.Key,
			"field":    marshalFieldRef(diagnostic.FieldRef),
			"owner":    string(diagnostic.Owner),
		})
	}
	return map[string]interface{}{
		"values":             values,
		"bindings":           bindings,
		"resolverAttempts":   marshalResolverAttempts(state.ResolverAttempts),
		"unresolvedFrontier": marshalUnresolvedFrontier(state.UnresolvedFrontier),
		"diagnostics":        diagnostics,
	}
}

func marshalUnresolvedFrontier(frontier model.UnresolvedFrontier) map[string]interface{} {
	needs := make([]map[string]interface{}, 0, len(frontier.Needs))
	for _, need := range frontier.Needs {
		ids := make([]string, 0, len(need.ResolverAttemptIDs))
		for _, id := range need.ResolverAttemptIDs {
			ids = append(ids, string(id))
		}
		item := map[string]interface{}{
			"id":                 string(need.ID),
			"field":              marshalFieldRef(need.FieldRef),
			"projectionKey":      string(need.ProjectionKey),
			"required":           need.Required,
			"blocking":           need.Blocking,
			"reason":             string(need.Reason),
			"description":        need.Description,
			"sensitivity":        string(need.Sensitivity),
			"exposure":           string(need.Exposure),
			"resolverAttemptIDs": ids,
		}
		if source := marshalSource(need.Source); source != nil {
			item["source"] = source
		}
		if source := marshalSource(need.Origin); source != nil {
			item["origin"] = source
		}
		needs = append(needs, item)
	}
	return map[string]interface{}{"needs": needs}
}

func marshalResolverAttempts(attempts []model.ResolverAttempt) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, marshalResolverAttempt(attempt))
	}
	return items
}

func marshalResolverAttempt(attempt model.ResolverAttempt) map[string]interface{} {
	diagnostics := make([]map[string]interface{}, 0, len(attempt.Diagnostics))
	for _, diagnostic := range attempt.Diagnostics {
		diagnostics = append(diagnostics, map[string]interface{}{
			"severity": string(diagnostic.Severity),
			"code":     diagnostic.Code,
			"message":  diagnostic.Message,
			"details":  append([]string{}, diagnostic.Details...),
			"key":      diagnostic.Key,
			"field":    marshalFieldRef(diagnostic.FieldRef),
			"owner":    string(diagnostic.Owner),
		})
	}
	item := map[string]interface{}{
		"id":            string(attempt.ID),
		"resolverID":    string(attempt.ResolverID),
		"field":         marshalFieldRef(attempt.FieldRef),
		"projectionKey": string(attempt.ProjectionKey),
		"outcome":       string(attempt.Outcome),
		"message":       attempt.Message,
		"startedAt":     timeString(attempt.StartedAt),
		"finishedAt":    timeString(attempt.FinishedAt),
		"diagnostics":   diagnostics,
	}
	if source := marshalSource(attempt.Source); source != nil {
		item["source"] = source
	}
	return item
}

func marshalResolverProposal(proposal resolver.Proposal) map[string]interface{} {
	return map[string]interface{}{
		"needID":        string(proposal.NeedID),
		"attemptID":     string(proposal.AttemptID),
		"resolverID":    string(proposal.ResolverID),
		"field":         marshalFieldRef(proposal.FieldRef),
		"projectionKey": string(proposal.ProjectionKey),
		"value":         marshalProposedValue(proposal.Value),
	}
}

func marshalProposedValue(value resolver.ProposedValue) map[string]interface{} {
	item := map[string]interface{}{
		"value":       value.Value,
		"sensitivity": string(value.Sensitivity),
		"exposure":    string(value.Exposure),
	}
	if source := marshalSource(value.Source); source != nil {
		item["source"] = source
	}
	return item
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
              source { name kind }
              origin { name kind }
              explicit
              confidence
              visibility
              exposure
              description
              updatedAt
              diagnostics { severity code message details key field owner }
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
			Source:      decodeSource(item["source"]),
			Origin:      decodeSource(item["origin"]),
			Explicit:    boolValue(item["explicit"]),
			Confidence:  model.BindingConfidence(stringValue(item["confidence"])),
			Visibility:  model.Visibility(stringValue(item["visibility"])),
			Exposure:    model.Exposure(stringValue(item["exposure"])),
			Description: stringValue(item["description"]),
			UpdatedAt:   timeValue(item["updatedAt"]),
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
		Checked:     intValue(row["checked"]),
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
		Exposure:    model.Exposure(stringValue(row["exposure"])),
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
		Provenance:   decodeStateProvenance(row["provenance"]),
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
	state.Diagnostics = decodeDiagnostics(row["diagnostics"])
	for _, item := range decodeList(row["resolverAttempts"]) {
		attempt, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		state.ResolverAttempts = append(state.ResolverAttempts, model.ResolverAttempt{
			ID:            model.ResolverAttemptID(stringValue(attempt["id"])),
			ResolverID:    model.ResolverID(stringValue(attempt["resolverID"])),
			FieldRef:      decodeFieldRef(attempt["field"]),
			ProjectionKey: model.ProjectionKey(stringValue(attempt["projectionKey"])),
			Outcome:       model.ResolverAttemptOutcome(stringValue(attempt["outcome"])),
			Message:       stringValue(attempt["message"]),
			Source:        decodeSource(attempt["source"]),
			StartedAt:     timeValue(attempt["startedAt"]),
			FinishedAt:    timeValue(attempt["finishedAt"]),
			Diagnostics:   decodeDiagnostics(attempt["diagnostics"]),
		})
	}
	state.UnresolvedFrontier = decodeUnresolvedFrontier(row["unresolvedFrontier"])
	return state
}

func decodeUnresolvedFrontier(raw interface{}) model.UnresolvedFrontier {
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

func decodeStateProvenance(raw interface{}) store.StateProvenance {
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
			Details:  decodeStringList(item["details"]),
			Key:      stringValue(item["key"]),
			FieldRef: decodeFieldRef(item["field"]),
			Owner:    model.DiagnosticOwner(stringValue(item["owner"])),
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
              exposure
              source { name kind }
              diagnostics { severity code message details key field owner }
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

const checkQuery = `
query OwlCheck($input: LoadInput!) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            check { ok checked diagnostics { severity code message details key field owner } }
          }
        }
      }
    }
  }
}`

const dotenvSpecQuery = `
query OwlDotenvSpec($input: LoadInput!) {
  Environment {
    load(input: $input) {
      normalize {
        validate {
          render {
            dotenvSpec
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
