package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/resolver"
)

func TestStoreSnapshotSourceAndCheck(t *testing.T) {
	t.Parallel()

	s, err := NewStore(
		WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		WithEnvSpec(".env.example", strings.NewReader("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, model.TypeCorePlain, byName["API_URL"].Type)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, model.TypeCoreSecret, byName["API_KEY"].Type)
	assert.Equal(t, "[hidden]", byName["DATABASE_URL"].Value)
	assert.Equal(t, model.TypeCoreOpaque, byName["DATABASE_URL"].Type)
	assert.Empty(t, byName["DATABASE_URL"].Diagnostics)
	assert.Equal(t, "[unset]", byName["MISSING_TOKEN"].Value)
	assert.Equal(t, model.VisibilityUnresolved, byName["MISSING_TOKEN"].Visibility)

	source, err := s.Source(SourcePolicy{Insecure: true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"DATABASE_URL=postgres://example",
	}, source)

	check := s.Check()
	assert.False(t, check.OK)
	require.NotEmpty(t, check.Diagnostics)
	assert.Equal(t, model.DiagnosticError, check.Diagnostics[len(check.Diagnostics)-1].Severity)
}

func TestStoreSnapshotOrdersExplicitBindingsBeforeInferredBindings(t *testing.T) {
	t.Parallel()

	s, err := NewStore(
		WithDotenv(".env", strings.NewReader("OMEGA=value\nAPPLE=value\nZETA=value\nBETA=value\n")),
		WithEnvSpec(".env.example", strings.NewReader("ZETA=\"Zeta\" # Plain\nBETA=\"Beta\" # Plain\n")),
	)
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{Reveal: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"ZETA", "BETA", "APPLE", "OMEGA"}, snapshotNames(snapshot))
	assert.True(t, snapshot[0].Explicit)
	assert.True(t, snapshot[1].Explicit)
	assert.False(t, snapshot[2].Explicit)
	assert.False(t, snapshot[3].Explicit)
}

func TestStoreTypeProposesMissingPrimitiveTypes(t *testing.T) {
	t.Parallel()

	s, err := NewStore(
		WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nSERVICE_HOST=localhost\nSERVICE_PORT=8080\nTARGET_PLATFORM=darwin/arm64\n")),
		WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain\n")),
	)
	require.NoError(t, err)

	result, err := s.Type(TypePolicy{All: true})
	require.NoError(t, err)

	require.Len(t, result.Proposals, 4)
	byKey := typeProposalsByKey(result.Proposals)
	assert.Equal(t, model.TypeCoreSecret, byKey["API_KEY"].SuggestedType)
	assert.Equal(t, "key name suggests sensitive value", byKey["API_KEY"].Reason)
	assert.Empty(t, byKey["SERVICE_HOST"].SuggestedType)
	assert.Equal(t, model.BindingConfidenceNone, byKey["SERVICE_HOST"].Confidence)
	assert.Empty(t, byKey["SERVICE_PORT"].SuggestedType)
	assert.Equal(t, model.BindingConfidenceNone, byKey["SERVICE_PORT"].Confidence)
	assert.Empty(t, byKey["TARGET_PLATFORM"].SuggestedType)
	assert.Equal(t, model.BindingConfidenceNone, byKey["TARGET_PLATFORM"].Confidence)
	assert.Equal(t, "no primitive type heuristic matched", byKey["TARGET_PLATFORM"].Reason)
	assert.NotContains(t, byKey, "API_URL")
}

func TestStoreTypeSkipsDefaultPlainProposalsByDefault(t *testing.T) {
	t.Parallel()

	s, err := NewStore(
		WithDotenv(".env", strings.NewReader("API_KEY=secret\nTARGET_PLATFORM=darwin/arm64\n")),
	)
	require.NoError(t, err)

	result, err := s.Type(TypePolicy{})
	require.NoError(t, err)

	require.Len(t, result.Proposals, 1)
	assert.Equal(t, "API_KEY", result.Proposals[0].Key)
	assert.Equal(t, model.TypeCoreSecret, result.Proposals[0].SuggestedType)
}

func TestRedisHostDiagnostics(t *testing.T) {
	t.Parallel()

	types := registry.NewBuiltInRegistry()
	ref := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "host"}
	assert.Empty(t, fieldValueDiagnostics(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "redis.internal",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
	}))

	diagnostics := fieldValueDiagnostics(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-host", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, `universe/redis.host value "" is invalid`)
	assert.Contains(t, diagnostics[0].Message, "must be an IP address or DNS hostname")

	diagnostics = fieldValueDiagnostics(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "not a host",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-host", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, `value "not a host" is invalid: must be an IP address or DNS hostname`)
}

func TestRedisPortDiagnostics(t *testing.T) {
	t.Parallel()

	types := registry.NewBuiltInRegistry()
	ref := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "port"}
	assert.Empty(t, fieldValueDiagnostics(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "6379",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
	}))

	diagnostics := fieldValueDiagnostics(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "not-a-port",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-port", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, `value "not-a-port" is invalid: must be an integer between 1 and 65535`)
}

func TestPrimitiveValueDiagnostics(t *testing.T) {
	t.Parallel()

	types := registry.NewBuiltInRegistry()
	urlRef := model.FieldRef{TypeID: model.TypeCoreURL, Instance: "default", Field: "service.url"}
	diagnostics := valueDiagnostics(types, model.TypeCoreURL, model.Value{
		FieldRef:    urlRef,
		Resolved:    "example.com",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-url", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, `value "example.com" is invalid: must be an absolute URL`)

	secretRef := model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"}
	diagnostics = valueDiagnostics(types, model.TypeCoreSecret, model.Value{
		FieldRef:    secretRef,
		Resolved:    "",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivitySensitive,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-secret", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, "core/secret value is invalid: must not be empty")
}

func TestStoreWithDotenv(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithDotenv("[process]", strings.NewReader("REDIS_HOST=localhost\nREDIS_PORT=6379\n")))
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, model.TypeCoreOpaque, byName["REDIS_HOST"].Type)
	assert.Equal(t, `core/opaque("default").redis.host`, byName["REDIS_HOST"].Field.String())
	assert.Equal(t, "[process]", byName["REDIS_HOST"].Source.Name)
}

func TestStorePreservesDotenvValueSource(t *testing.T) {
	t.Parallel()

	s, err := NewStore(
		WithDotenv("[process]", strings.NewReader("PROCESS_ONLY=from-process\nDUPLICATE_KEY=from-process\n")),
		WithDotenv(".env", strings.NewReader("FILE_ONLY=from-file\nDUPLICATE_KEY=from-file\n")),
	)
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, "from-process", byName["PROCESS_ONLY"].Value)
	assert.Equal(t, "[process]", byName["PROCESS_ONLY"].Source.Name)
	assert.Equal(t, "from-file", byName["FILE_ONLY"].Value)
	assert.Equal(t, ".env", byName["FILE_ONLY"].Source.Name)
	assert.Equal(t, "from-file", byName["DUPLICATE_KEY"].Value)
	assert.Equal(t, ".env", byName["DUPLICATE_KEY"].Source.Name)
}

func TestStoreRecordsFactOperationsOnly(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\n")))
	require.NoError(t, err)

	records := s.OperationRecords()
	require.Len(t, records, 1)
	assert.Equal(t, OperationRecordLoad, records[0].Kind)
	assert.Equal(t, ".env", records[0].Load.DotenvSource.Name)
	assert.Equal(t, []DotenvVariable{{Key: "API_URL", Value: "https://api.example.com", Source: model.Source{Name: ".env", Kind: "dotenv"}}}, records[0].Load.Dotenv)

	_, err = s.Apply(context.Background(), UpdateOperation{
		Source: model.Source{Name: "[update]", Kind: "dotenv"},
		Dotenv: []DotenvVariable{{Key: "API_URL", Value: "https://next.example.com"}},
	})
	require.NoError(t, err)
	_, err = s.Apply(context.Background(), DeleteOperation{Keys: []string{"API_URL"}})
	require.NoError(t, err)

	records = s.OperationRecords()
	require.Len(t, records, 3)
	assert.Equal(t, OperationRecordUpdate, records[1].Kind)
	assert.Equal(t, OperationRecordDelete, records[2].Kind)
	assert.Equal(t, []string{"API_URL"}, records[2].Delete.Keys)
}

func TestStoreRecordsResolverAttemptWithoutMutatingValues(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\n")))
	require.NoError(t, err)
	before := s.State()
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
	}

	after, err := s.Apply(context.Background(), RecordResolverAttemptOperation{Attempt: attempt})
	require.NoError(t, err)

	assert.Equal(t, before.Values, after.Values)
	assert.Equal(t, before.Bindings, after.Bindings)
	require.Len(t, after.ResolverAttempts, 1)
	assert.Equal(t, attempt, after.ResolverAttempts[0])

	records := s.OperationRecords()
	require.Len(t, records, 2)
	assert.Equal(t, OperationRecordResolverAttempt, records[1].Kind)
	assert.Equal(t, attempt, records[1].ResolverAttempt)
}

func TestStoreAppliesResolverProposalThroughStateOperation(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithEnvSpec(".env.example", strings.NewReader("API_KEY=\"API key\" # Secret!\n")))
	require.NoError(t, err)
	before := s.State()
	require.Len(t, before.UnresolvedFrontier.Needs, 1)
	need := before.UnresolvedFrontier.Needs[0]
	timestamp := time.Date(2026, 8, 3, 20, 30, 0, 0, time.UTC)

	after, err := s.Apply(context.Background(), ApplyResolverProposalOperation{
		Timestamp: timestamp,
		Proposal: resolver.Proposal{
			NeedID:        need.ID,
			AttemptID:     "attempt-000001",
			ResolverID:    "core/dotenv",
			FieldRef:      need.FieldRef,
			ProjectionKey: need.ProjectionKey,
			Value: resolver.ProposedValue{
				Value:       "resolved-secret",
				Source:      model.Source{Name: ".env", Kind: "dotenv"},
				Sensitivity: model.SensitivitySensitive,
				Exposure:    model.ExposureClear,
			},
		},
	})
	require.NoError(t, err)
	after, err = s.Apply(context.Background(), IntegrityOperation{})
	require.NoError(t, err)

	value := after.Values[need.FieldRef]
	assert.Equal(t, "resolved-secret", value.Resolved)
	assert.Equal(t, model.VisibilityLiteral, value.Visibility)
	assert.Equal(t, model.Source{Name: ".env", Kind: "dotenv"}, value.Source)
	assert.Equal(t, model.OperationID("resolve:attempt-000001"), value.LastOperationID)
	assert.Empty(t, after.UnresolvedFrontier.Needs)
	require.NotEmpty(t, after.Operations)
	operation := after.Operations[len(after.Operations)-1]
	assert.Equal(t, model.OperationKindResolve, operation.Kind)
	assert.Equal(t, model.OperationID("resolve:attempt-000001"), operation.ID)
	assert.Equal(t, timestamp, operation.Timestamp)

	records := s.OperationRecords()
	require.Len(t, records, 2)
	assert.Equal(t, OperationRecordApplyResolverProposal, records[1].Kind)
	assert.Equal(t, model.ResolverAttemptID("attempt-000001"), records[1].ResolverProposal.AttemptID)
}

func TestStoreProposalApplicationLeavesInvalidValuesForIntegrity(t *testing.T) {
	t.Parallel()

	ref := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "port"}
	state := model.NewEffectiveState()
	state.Bindings = []model.Binding{{
		FieldRef:     ref,
		ProjectionID: model.ProjectionDotenv,
		Key:          "REDIS_PORT",
		Explicit:     true,
		Required:     true,
	}}
	state.Values[ref] = model.Value{
		FieldRef:    ref,
		Visibility:  model.VisibilityUnresolved,
		Sensitivity: model.SensitivityPlaintext,
		Exposure:    model.ExposureClear,
	}
	state.UnresolvedFrontier = BuildUnresolvedFrontier(state)
	s := NewState(state, registry.NewBuiltInRegistry())

	after, err := s.Apply(context.Background(), ApplyResolverProposalOperation{
		Proposal: resolver.Proposal{
			NeedID:        state.UnresolvedFrontier.Needs[0].ID,
			AttemptID:     "attempt-000001",
			ResolverID:    "core/dotenv",
			FieldRef:      ref,
			ProjectionKey: "REDIS_PORT",
			Value: resolver.ProposedValue{
				Value:       "not-a-port",
				Source:      model.Source{Name: ".env", Kind: "dotenv"},
				Sensitivity: model.SensitivityPlaintext,
				Exposure:    model.ExposureClear,
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "not-a-port", after.Values[ref].Resolved)

	after, err = s.Apply(context.Background(), IntegrityOperation{Types: registry.NewBuiltInRegistry()})
	require.NoError(t, err)
	require.NotEmpty(t, after.Diagnostics)
	assert.Equal(t, "type.invalid-port", after.Diagnostics[0].Code)
	require.Len(t, after.UnresolvedFrontier.Needs, 1)
	assert.Equal(t, model.UnresolvedReasonInvalid, after.UnresolvedFrontier.Needs[0].Reason)
}

func TestBuildUnresolvedFrontierIncludesExplicitOptionalAndRequiredNeeds(t *testing.T) {
	t.Parallel()

	requiredRef := model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"}
	optionalRef := model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "api.url"}
	inferredRef := model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: "token"}
	state := model.NewEffectiveState()
	state.Bindings = []model.Binding{
		{
			FieldRef:    requiredRef,
			Key:         "API_KEY",
			Description: "API key",
			Source:      model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
			Origin:      model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
			Explicit:    true,
			Required:    true,
		},
		{
			FieldRef:    optionalRef,
			Key:         "API_URL",
			Description: "API URL",
			Source:      model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
			Origin:      model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
			Explicit:    true,
		},
		{
			FieldRef: inferredRef,
			Key:      "TOKEN",
			Explicit: false,
		},
	}
	state.Values[requiredRef] = model.Value{
		FieldRef:    requiredRef,
		Visibility:  model.VisibilityUnresolved,
		Sensitivity: model.SensitivitySensitive,
		Exposure:    model.ExposureClear,
	}
	state.Values[optionalRef] = model.Value{
		FieldRef:    optionalRef,
		Visibility:  model.VisibilityUnresolved,
		Sensitivity: model.SensitivityPlaintext,
		Exposure:    model.ExposureClear,
	}
	state.Values[inferredRef] = model.Value{
		FieldRef:    inferredRef,
		Visibility:  model.VisibilityUnresolved,
		Sensitivity: model.SensitivityUnknown,
		Exposure:    model.ExposureOpaque,
	}
	state.ResolverAttempts = []model.ResolverAttempt{
		{
			ID:            "attempt-000001",
			ResolverID:    "core/dotenv",
			FieldRef:      requiredRef,
			ProjectionKey: "API_KEY",
			Outcome:       model.ResolverAttemptNotFound,
		},
	}

	frontier := BuildUnresolvedFrontier(state)

	require.Len(t, frontier.Needs, 2)
	assert.Equal(t, "API_KEY", string(frontier.Needs[0].ProjectionKey))
	assert.True(t, frontier.Needs[0].Required)
	assert.True(t, frontier.Needs[0].Blocking)
	assert.Equal(t, model.UnresolvedReasonMissing, frontier.Needs[0].Reason)
	assert.Equal(t, []model.ResolverAttemptID{"attempt-000001"}, frontier.Needs[0].ResolverAttemptIDs)
	assert.Equal(t, "API_URL", string(frontier.Needs[1].ProjectionKey))
	assert.False(t, frontier.Needs[1].Required)
	assert.False(t, frontier.Needs[1].Blocking)
}

func TestBuildUnresolvedFrontierIncludesInvalidExplicitValuesOnly(t *testing.T) {
	t.Parallel()

	invalidRef := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "port"}
	hiddenRef := model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: "database.url"}
	state := model.NewEffectiveState()
	state.Bindings = []model.Binding{
		{
			FieldRef: invalidRef,
			Key:      "REDIS_PORT",
			Explicit: true,
		},
		{
			FieldRef: hiddenRef,
			Key:      "DATABASE_URL",
			Explicit: true,
		},
	}
	state.Values[invalidRef] = model.Value{
		FieldRef:    invalidRef,
		Resolved:    "not-a-port",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityPlaintext,
		Exposure:    model.ExposureClear,
	}
	state.Values[hiddenRef] = model.Value{
		FieldRef:    hiddenRef,
		Resolved:    "postgres://example",
		Visibility:  model.VisibilityHidden,
		Sensitivity: model.SensitivityUnknown,
		Exposure:    model.ExposureOpaque,
	}
	state.Diagnostics = []model.Diagnostic{
		{
			Severity: model.DiagnosticError,
			Code:     "type.invalid-port",
			Key:      "REDIS_PORT",
			FieldRef: invalidRef,
			Owner:    model.DiagnosticOwnerValidation,
		},
	}

	frontier := BuildUnresolvedFrontier(state)

	require.Len(t, frontier.Needs, 1)
	assert.Equal(t, "REDIS_PORT", string(frontier.Needs[0].ProjectionKey))
	assert.Equal(t, model.UnresolvedReasonInvalid, frontier.Needs[0].Reason)
}

func snapshotByName(items []SnapshotItem) map[string]SnapshotItem {
	result := make(map[string]SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}

func snapshotNames(items []SnapshotItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Name)
	}
	return result
}

func typeProposalsByKey(items []TypeProposal) map[string]TypeProposal {
	result := make(map[string]TypeProposal, len(items))
	for _, item := range items {
		result[item.Key] = item
	}
	return result
}
