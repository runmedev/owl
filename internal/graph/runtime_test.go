package graph

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/store"
)

func TestRuntimeDrivesLoadNormalizeValidateSnapshot(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	items, err := runtime.Snapshot(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv: []store.DotenvVariable{
			{Key: "API_URL", Value: "https://api.example.com"},
			{Key: "API_KEY", Value: "secret"},
			{Key: "REDIS_HOST", Value: "localhost"},
		},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
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

	envs, err := runtime.Dotenv(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv: []store.DotenvVariable{
			{Key: "API_KEY", Value: "secret"},
			{Key: "API_URL", Value: "https://api.example.com"},
		},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: "package.json", Kind: "package-json"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
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

	check, err := runtime.Check(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
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

func TestPlanStateEnvelopeQueryStacksOperationRecords(t *testing.T) {
	t.Parallel()

	plan, err := planStateEnvelopeQuery([]store.OperationRecord{
		{
			Kind: store.OperationRecordLoad,
			Load: store.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Dotenv:       []store.DotenvVariable{{Key: "API_URL", Value: "https://api.example.com"}},
			},
		},
		{
			Kind: store.OperationRecordUpdate,
			Update: store.UpdateOperation{
				Source: model.Source{Name: "[update]", Kind: "dotenv"},
				Dotenv: []store.DotenvVariable{{Key: "API_URL", Value: "https://next.example.com"}},
			},
		},
		{
			Kind:   store.OperationRecordDelete,
			Delete: store.DeleteOperation{Keys: []string{"API_KEY"}},
		},
		{
			Kind: store.OperationRecordResolverAttempt,
			ResolverAttempt: model.ResolverAttempt{
				ID:            "attempt-000001",
				ResolverID:    "core/dotenv",
				FieldRef:      model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
				ProjectionKey: "API_KEY",
				Outcome:       model.ResolverAttemptNotFound,
			},
		},
	})
	require.NoError(t, err)

	assert.Contains(t, plan.Query, "$load_0: LoadInput!")
	assert.Contains(t, plan.Query, "$update_1: DotenvInput")
	assert.Contains(t, plan.Query, "$delete_2: [String!]")
	assert.Contains(t, plan.Query, "$resolverAttempt_3: ResolverAttemptInput!")
	assert.Contains(t, plan.Query, "load(input: $load_0)")
	assert.Contains(t, plan.Query, "update(dotenv: $update_1)")
	assert.Contains(t, plan.Query, "delete(keys: $delete_2)")
	assert.Contains(t, plan.Query, "recordResolverAttempt(attempt: $resolverAttempt_3)")
	assert.Contains(t, plan.Query, "unresolvedFrontier")
	assert.NotContains(t, plan.Query, "reconcile")
	assert.Equal(t, []string{"load", "update", "delete", "recordResolverAttempt", "normalize", "validate", "state", "envelope"}, plan.Path)
}

func TestRuntimeMaterializesStateEnvelopeFromOperationRecords(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	envelope, err := runtime.StateEnvelopeForOperations(context.Background(), []store.OperationRecord{
		{
			Kind: store.OperationRecordLoad,
			Load: store.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Dotenv: []store.DotenvVariable{
					{Key: "API_URL", Value: "https://api.example.com"},
					{Key: "API_KEY", Value: "secret"},
				},
				Contracts: []store.EnvContract{
					{
						Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
						Projection: model.ProjectionDotenv,
						Bindings: []store.EnvBinding{
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
			Kind: store.OperationRecordUpdate,
			Update: store.UpdateOperation{
				Source: model.Source{Name: "[update]", Kind: "dotenv"},
				Dotenv: []store.DotenvVariable{{Key: "API_URL", Value: "https://next.example.com"}},
			},
		},
		{
			Kind:   store.OperationRecordDelete,
			Delete: store.DeleteOperation{Keys: []string{"API_KEY"}},
		},
	})
	require.NoError(t, err)

	s := store.NewState(envelope.State, nil)
	got, ok, err := s.Get("API_URL", store.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://next.example.com", got.Value)
	assert.False(t, envelope.State.Values[got.Field].UpdatedAt.IsZero())
	_, ok, err = s.Get("API_KEY", store.GetPolicy{Reveal: true})
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

	envelope, err := runtime.StateEnvelopeForOperations(context.Background(), []store.OperationRecord{
		{
			Kind: store.OperationRecordLoad,
			Load: store.LoadInput{
				DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
				Dotenv:       []store.DotenvVariable{{Key: "API_URL", Value: "https://api.example.com"}},
			},
		},
		{
			Kind:            store.OperationRecordResolverAttempt,
			ResolverAttempt: attempt,
		},
	})
	require.NoError(t, err)

	require.Len(t, envelope.State.ResolverAttempts, 1)
	assert.Equal(t, attempt, envelope.State.ResolverAttempts[0])
	assert.NotContains(t, envelope.State.ResolverAttempts[0].Message, "secret")

	s := store.NewState(envelope.State, nil)
	got, ok, err := s.Get("API_URL", store.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://api.example.com", got.Value)
}

func TestRuntimeMaterializesUnresolvedFrontier(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	envelope, err := runtime.StateEnvelope(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv:       []store.DotenvVariable{{Key: "PRESENT_SECRET", Value: "secret"}},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: "owl.toml", Kind: "owl-config"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
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
