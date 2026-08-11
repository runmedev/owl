package graph

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/resolver"
	"github.com/runmedev/owl/internal/state"
)

func TestRuntimeDrivesLoadNormalizeValidateSnapshot(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	items, err := runtime.Snapshot(context.Background(), state.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv: []state.DotenvVariable{
			{Key: "API_URL", Value: "https://api.example.com"},
			{Key: "API_KEY", Value: "secret"},
			{Key: "REDIS_HOST", Value: "localhost"},
		},
		Contracts: []state.EnvContract{
			{
				Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
				Projection: model.ProjectionDotenv,
				Bindings: []state.EnvBinding{
					{
						Key:         "API_URL",
						FieldRef:    model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "api.url"},
						Projection:  model.ProjectionDotenv,
						Description: "API URL",
						Source:      model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
					},
					{
						Key:        "API_KEY",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
						Projection: model.ProjectionDotenv,
						Required:   true,
						Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
					},
				},
			},
		},
	}, SnapshotPolicy{})
	require.NoError(t, err)

	byName := snapshotByName(items)
	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, model.TypeCorePlain, byName["API_URL"].Type)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, model.TypeCoreSecret, byName["API_KEY"].Type)
	assert.Equal(t, `core/plain("default").api.url`, byName["API_URL"].Field.String())
	assert.Equal(t, model.TypeCoreOpaque, byName["REDIS_HOST"].Type)
}

func TestRuntimeRendersDotenvThroughGraphQL(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	envs, err := runtime.Dotenv(context.Background(), state.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv: []state.DotenvVariable{
			{Key: "API_KEY", Value: "secret"},
			{Key: "API_URL", Value: "https://api.example.com"},
		},
		Contracts: []state.EnvContract{
			{
				Source:     model.Source{Name: "package.json", Kind: "package-json"},
				Projection: model.ProjectionDotenv,
				Bindings: []state.EnvBinding{
					{
						Key:        "API_KEY",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
						Projection: model.ProjectionDotenv,
					},
					{
						Key:        "API_URL",
						FieldRef:   model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "api.url"},
						Projection: model.ProjectionDotenv,
					},
				},
			},
		},
	}, DotenvPolicy{Insecure: true})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
	}, envs)
}

func TestRuntimeCheckReportsRequiredDiagnostics(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	check, err := runtime.Check(context.Background(), state.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Contracts: []state.EnvContract{
			{
				Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
				Projection: model.ProjectionDotenv,
				Bindings: []state.EnvBinding{
					{
						Key:        "API_KEY",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
						Projection: model.ProjectionDotenv,
						Required:   true,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "dotenv.unresolved-required")
}

func TestRuntimeSchemaUsesVisibilityAndExposureNames(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	schemaJSON, err := runtime.SchemaJSON(context.Background())
	require.NoError(t, err)

	for _, name := range []string{
		`"name": "StateValue"`,
		`"name": "StateValueInput"`,
		`"name": "ResolverAttempt"`,
		`"name": "ResolverAttemptInput"`,
		`"name": "UnresolvedFrontier"`,
		`"name": "UnresolvedFrontierInput"`,
		`"name": "UnresolvedNeed"`,
		`"name": "UnresolvedNeedInput"`,
		`"name": "SnapshotItem"`,
		`"name": "GetResult"`,
		`"name": "visibility"`,
		`"name": "exposure"`,
		`"name": "resolverAttempts"`,
		`"name": "unresolvedFrontier"`,
		`"name": "createdAt"`,
		`"name": "updatedAt"`,
	} {
		assert.Contains(t, schemaJSON, name)
	}
	for _, oldName := range []string{
		`"name": "status"`,
		`"name": "semanticVisibility"`,
		`"name": "effectiveVisibility"`,
	} {
		assert.NotContains(t, schemaJSON, oldName)
	}
}

func TestRuntimeSchemaInputFieldsMatchBoundaryDescriptors(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	schemaJSON, err := runtime.SchemaJSON(context.Background())
	require.NoError(t, err)

	fieldsByInput := introspectionInputFields(t, schemaJSON)
	for inputName, want := range map[string][]string{
		"SourceInput":             {"kind", "name"},
		"FieldRefInput":           {"field", "instance", "typeID"},
		"DotenvVariableInput":     {"key", "source", "value"},
		"DotenvInput":             {"source", "timestamp", "variables"},
		"EnvBindingInput":         {"description", "exposure", "field", "key", "order", "projection", "required", "sensitivity", "source"},
		"EnvContractInput":        {"bindings", "projection", "source"},
		"DiagnosticInput":         {"code", "details", "field", "key", "message", "owner", "severity"},
		"ResolverAttemptInput":    {"diagnostics", "field", "finishedAt", "id", "message", "outcome", "projectionKey", "resolverID", "source", "startedAt"},
		"ResolverProposalInput":   {"attemptID", "field", "needID", "projectionKey", "resolverID", "value"},
		"StateEnvelopeInput":      {"modelVersion", "provenance", "state"},
		"LoadInput":               {"contracts", "dotenv", "envelope", "timestamp"},
		"OperationMetadataInput":  {"actor", "id", "kind", "projection", "source", "timestamp"},
		"StateProvenanceInput":    {"operations", "sources"},
		"EffectiveStateInput":     {"bindings", "diagnostics", "resolverAttempts", "unresolvedFrontier", "values"},
		"UnresolvedFrontierInput": {"needs"},
	} {
		got, ok := fieldsByInput[inputName]
		require.True(t, ok, "missing input %s", inputName)
		assert.Equal(t, want, got, inputName)
	}
}

func introspectionInputFields(t *testing.T, schemaJSON string) map[string][]string {
	t.Helper()

	var payload struct {
		Schema struct {
			Types []struct {
				Name        string `json:"name"`
				InputFields []struct {
					Name string `json:"name"`
				} `json:"inputFields"`
			} `json:"types"`
		} `json:"__schema"`
	}
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), &payload))

	fieldsByInput := make(map[string][]string)
	for _, typ := range payload.Schema.Types {
		if len(typ.InputFields) == 0 {
			continue
		}
		fields := make([]string, 0, len(typ.InputFields))
		for _, field := range typ.InputFields {
			fields = append(fields, field.Name)
		}
		sort.Strings(fields)
		fieldsByInput[typ.Name] = fields
	}
	return fieldsByInput
}

func TestPlanStateEnvelopeQueryStacksOperationRecords(t *testing.T) {
	t.Parallel()

	plan, err := planStateEnvelopeQuery([]state.OperationRecord{
		{
			Kind: state.OperationRecordLoad,
			Load: state.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Dotenv:       []state.DotenvVariable{{Key: "API_URL", Value: "https://api.example.com"}},
			},
		},
		{
			Kind: state.OperationRecordUpdate,
			Update: state.UpdateOperation{
				Source: model.Source{Name: "[update]", Kind: "dotenv"},
				Dotenv: []state.DotenvVariable{{Key: "API_URL", Value: "https://next.example.com"}},
			},
		},
		{
			Kind:   state.OperationRecordDelete,
			Delete: state.DeleteOperation{Keys: []string{"API_KEY"}},
		},
		{
			Kind: state.OperationRecordResolverAttempt,
			ResolverAttempt: model.ResolverAttempt{
				ID:            "attempt-000001",
				ResolverID:    "core/dotenv",
				FieldRef:      model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
				ProjectionKey: "API_KEY",
				Outcome:       model.ResolverAttemptNotFound,
			},
		},
		{
			Kind: state.OperationRecordApplyResolverProposal,
			ResolverProposal: resolver.Proposal{
				AttemptID:     "attempt-000002",
				ResolverID:    "core/dotenv",
				FieldRef:      model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
				ProjectionKey: "API_KEY",
				Value: resolver.ProposedValue{
					Value:       "secret",
					Source:      model.Source{Name: ".env", Kind: "dotenv"},
					Sensitivity: model.SensitivitySensitive,
					Exposure:    model.ExposureClear,
				},
			},
		},
	})
	require.NoError(t, err)

	assert.Contains(t, plan.Query, "$load_0: LoadInput!")
	assert.Contains(t, plan.Query, "$update_1: DotenvInput")
	assert.Contains(t, plan.Query, "$delete_2: [String!]")
	assert.Contains(t, plan.Query, "$resolverAttempt_3: ResolverAttemptInput!")
	assert.Contains(t, plan.Query, "$resolverProposal_4: ResolverProposalInput!")
	assert.Contains(t, plan.Query, "load(input: $load_0)")
	assert.Contains(t, plan.Query, "update(dotenv: $update_1)")
	assert.Contains(t, plan.Query, "delete(keys: $delete_2)")
	assert.Contains(t, plan.Query, "recordResolverAttempt(attempt: $resolverAttempt_3)")
	assert.Contains(t, plan.Query, "applyResolverProposal(proposal: $resolverProposal_4")
	assert.Contains(t, plan.Query, "unresolvedFrontier")
	assert.NotContains(t, plan.Query, "reconcile")
	assert.Equal(t, []string{"load", "update", "delete", "recordResolverAttempt", "applyResolverProposal", "normalize", "validate", "state", "envelope"}, plan.Path)
}

func TestRuntimeMaterializesStateEnvelopeFromOperationRecords(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	envelope, err := runtime.StateEnvelopeForOperations(context.Background(), []state.OperationRecord{
		{
			Kind: state.OperationRecordLoad,
			Load: state.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Dotenv: []state.DotenvVariable{
					{Key: "API_URL", Value: "https://api.example.com"},
					{Key: "API_KEY", Value: "secret"},
				},
				Contracts: []state.EnvContract{
					{
						Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
						Projection: model.ProjectionDotenv,
						Bindings: []state.EnvBinding{
							{
								Key:        "API_KEY",
								FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
								Projection: model.ProjectionDotenv,
								Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
							},
						},
					},
				},
			},
		},
		{
			Kind: state.OperationRecordUpdate,
			Update: state.UpdateOperation{
				Source: model.Source{Name: "[update]", Kind: "dotenv"},
				Dotenv: []state.DotenvVariable{{Key: "API_URL", Value: "https://next.example.com"}},
			},
		},
		{
			Kind:   state.OperationRecordDelete,
			Delete: state.DeleteOperation{Keys: []string{"API_KEY"}},
		},
	})
	require.NoError(t, err)

	s := state.MachineFromState(envelope.State, nil)
	got, ok, err := s.Get("API_URL", state.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://next.example.com", got.Value)
	assert.False(t, envelope.State.Values[got.Field].UpdatedAt.IsZero())
	_, ok, err = s.Get("API_KEY", state.GetPolicy{Reveal: true})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRuntimeMaterializesResolverAttemptsFromOperationRecords(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)
	startedAt := time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)
	attempt := model.ResolverAttempt{
		ID:            "attempt-000001",
		ResolverID:    "core/dotenv",
		FieldRef:      model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
		ProjectionKey: "API_KEY",
		Outcome:       model.ResolverAttemptNotFound,
		Message:       "dotenv value was not present",
		Source:        model.Source{Name: ".env", Kind: "dotenv"},
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		Diagnostics: []model.Diagnostic{
			{
				Severity: model.DiagnosticInfo,
				Code:     "resolver.not-found",
				Message:  "resolver did not find a value",
				Key:      "API_KEY",
				FieldRef: model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
				Owner:    model.DiagnosticOwnerValidation,
			},
		},
	}

	envelope, err := runtime.StateEnvelopeForOperations(context.Background(), []state.OperationRecord{
		{
			Kind: state.OperationRecordLoad,
			Load: state.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Dotenv:       []state.DotenvVariable{{Key: "API_URL", Value: "https://api.example.com"}},
			},
		},
		{
			Kind:            state.OperationRecordResolverAttempt,
			ResolverAttempt: attempt,
		},
	})
	require.NoError(t, err)

	require.Len(t, envelope.State.ResolverAttempts, 1)
	assert.Equal(t, attempt, envelope.State.ResolverAttempts[0])
	assert.NotContains(t, envelope.State.ResolverAttempts[0].Message, "secret")

	s := state.MachineFromState(envelope.State, nil)
	got, ok, err := s.Get("API_URL", state.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://api.example.com", got.Value)
}

func TestRuntimeMaterializesResolverProposalsFromOperationRecords(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)
	ref := model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"}
	timestamp := time.Date(2026, 8, 3, 20, 30, 0, 0, time.UTC)

	envelope, err := runtime.StateEnvelopeForOperations(context.Background(), []state.OperationRecord{
		{
			Kind: state.OperationRecordLoad,
			Load: state.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Contracts: []state.EnvContract{{
					Source:     model.Source{Name: ".env.example", Kind: "dotenv-spec"},
					Projection: model.ProjectionDotenv,
					Bindings: []state.EnvBinding{{
						Key:        "API_KEY",
						FieldRef:   ref,
						Projection: model.ProjectionDotenv,
						Required:   true,
					}},
				}},
			},
		},
		{
			Kind:      state.OperationRecordApplyResolverProposal,
			Timestamp: timestamp,
			ResolverProposal: resolver.Proposal{
				AttemptID:     "attempt-000001",
				ResolverID:    "core/dotenv",
				FieldRef:      ref,
				ProjectionKey: "API_KEY",
				Value: resolver.ProposedValue{
					Value:       "secret",
					Source:      model.Source{Name: ".env", Kind: "dotenv"},
					Sensitivity: model.SensitivitySensitive,
					Exposure:    model.ExposureClear,
				},
			},
		},
	})
	require.NoError(t, err)

	s := state.MachineFromState(envelope.State, nil)
	got, ok, err := s.Get("API_KEY", state.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "secret", got.Value)
	assert.Empty(t, envelope.State.UnresolvedFrontier.Needs)
	require.NotEmpty(t, envelope.Provenance.Operations)
	operation := envelope.Provenance.Operations[len(envelope.Provenance.Operations)-1]
	assert.Equal(t, model.OperationKindResolve, operation.Kind)
	assert.Equal(t, timestamp, operation.Timestamp)
}

func TestRuntimeMaterializesUnresolvedFrontier(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	envelope, err := runtime.StateEnvelope(context.Background(), state.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv:       []state.DotenvVariable{{Key: "PRESENT_SECRET", Value: "secret"}},
		Contracts: []state.EnvContract{
			{
				Source:     model.Source{Name: "owl.toml", Kind: "owl-config"},
				Projection: model.ProjectionDotenv,
				Bindings: []state.EnvBinding{
					{
						Key:         "MISSING_SECRET",
						FieldRef:    model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "missing.secret"},
						Projection:  model.ProjectionDotenv,
						Description: "Missing secret",
						Source:      model.Source{Name: "owl.toml", Kind: "owl-config"},
						Required:    true,
						Sensitivity: model.SensitivitySensitive,
						Exposure:    model.ExposureClear,
					},
					{
						Key:         "OPTIONAL_URL",
						FieldRef:    model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "optional.url"},
						Projection:  model.ProjectionDotenv,
						Description: "Optional URL",
						Source:      model.Source{Name: "owl.toml", Kind: "owl-config"},
						Sensitivity: model.SensitivityPlaintext,
						Exposure:    model.ExposureClear,
					},
					{
						Key:        "PRESENT_SECRET",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "present.secret"},
						Projection: model.ProjectionDotenv,
						Source:     model.Source{Name: "owl.toml", Kind: "owl-config"},
						Required:   true,
					},
				},
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, envelope.State.UnresolvedFrontier.Needs, 2)
	byKey := unresolvedNeedsByKey(envelope.State.UnresolvedFrontier.Needs)
	require.Contains(t, byKey, "MISSING_SECRET")
	assert.True(t, byKey["MISSING_SECRET"].Blocking)
	assert.Equal(t, model.UnresolvedReasonMissing, byKey["MISSING_SECRET"].Reason)
	require.Contains(t, byKey, "OPTIONAL_URL")
	assert.False(t, byKey["OPTIONAL_URL"].Blocking)
	assert.Equal(t, model.UnresolvedReasonMissing, byKey["OPTIONAL_URL"].Reason)
	assert.NotContains(t, byKey, "PRESENT_SECRET")
}

func snapshotByName(items []SnapshotItem) map[string]SnapshotItem {
	result := make(map[string]SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}

func diagnosticCodes(diagnostics []model.Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func unresolvedNeedsByKey(needs []model.UnresolvedNeed) map[string]model.UnresolvedNeed {
	result := make(map[string]model.UnresolvedNeed, len(needs))
	for _, need := range needs {
		result[string(need.ProjectionKey)] = need
	}
	return result
}
