package store

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
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

func TestValidateRedisHost(t *testing.T) {
	t.Parallel()

	types := registry.NewBuiltInRegistry()
	ref := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "host"}
	assert.Empty(t, validateFieldValue(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "redis.internal",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityNonSensitive,
	}))

	diagnostics := validateFieldValue(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityNonSensitive,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-host", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, "universe/redis.host value is invalid")

	diagnostics = validateFieldValue(types, model.TypeCorePlain, model.Value{
		FieldRef:    ref,
		Resolved:    "not a host",
		Visibility:  model.VisibilityLiteral,
		Sensitivity: model.SensitivityNonSensitive,
	})
	require.NotEmpty(t, diagnostics)
	assert.Equal(t, "type.invalid-host", diagnostics[0].Code)
	assert.Contains(t, diagnostics[0].Message, "not a host")
}

func TestStoreWithDotenv(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithDotenv("[system]", strings.NewReader("REDIS_HOST=localhost\nREDIS_PORT=6379\n")))
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, model.TypeUniverseRedis, byName["REDIS_HOST"].Type)
	assert.Equal(t, `universe/redis("default").host`, byName["REDIS_HOST"].Field.String())
}

func TestStoreRecordsFactOperationsOnly(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\n")))
	require.NoError(t, err)

	records := s.OperationRecords()
	require.Len(t, records, 1)
	assert.Equal(t, OperationRecordLoad, records[0].Kind)
	assert.Equal(t, ".env", records[0].Load.DotenvSource.Name)
	assert.Equal(t, []DotenvVariable{{Key: "API_URL", Value: "https://api.example.com"}}, records[0].Load.Dotenv)

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
