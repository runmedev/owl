package owl_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/pkg/owl"
)

func TestV2PublicAPI(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nREDIS_PASSWORD=hunter2\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	assert.Equal(t, "[masked]", snapshotByName(snapshot)["API_KEY"].Value)

	envs := dotenvLines(t, store, owl.DotenvPolicy{Insecure: true})
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"REDIS_PASSWORD=hunter2",
	}, envs)

	got, ok, err := store.Get(context.Background(), owl.GetInput{Key: "API_KEY"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[masked]", got.Value)

	keys, err := store.SensitiveKeys(context.Background(), owl.SensitiveKeysInput{})
	require.NoError(t, err)
	assert.Equal(t, []string{"API_KEY"}, keys.Keys)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "owl.store.v2", envelope.ModelVersion)

	next, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)
	applyUpdateLines(context.Background(), t, next, owl.Source{Name: "[override]", Kind: "dotenv"}, []string{"API_URL=https://next.example.com"}, nil)

	got, ok, err = next.Get(context.Background(), owl.GetInput{Key: "API_URL", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://next.example.com", got.Value)

	applyUpdateLines(context.Background(), t, next, owl.Source{Name: "[update]", Kind: "dotenv"}, nil, []string{"API_KEY"})
	_, ok, err = next.Get(context.Background(), owl.GetInput{Key: "API_KEY", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	assert.False(t, ok)
}

func snapshotItems(t *testing.T, store *owl.Store, policy owl.SnapshotPolicy) []owl.SnapshotItem {
	t.Helper()
	output, err := store.Snapshot(context.Background(), owl.SnapshotInput{
		Policy: policy,
		Filter: owl.SnapshotFilter{All: true},
	})
	require.NoError(t, err)
	return output.Envs
}

func dotenvLines(t *testing.T, store *owl.Store, policy owl.DotenvPolicy) []string {
	t.Helper()
	output, err := store.Source(context.Background(), owl.SourceInput{Policy: policy})
	require.NoError(t, err)
	return output.Envs
}

func checkStore(t *testing.T, store *owl.Store) owl.CheckOutput {
	t.Helper()
	output, err := store.Check(context.Background(), owl.CheckInput{})
	require.NoError(t, err)
	return output
}

func applyUpdateLines(ctx context.Context, t *testing.T, store *owl.Store, source owl.Source, lines []string, deleted []string) {
	t.Helper()
	var vars []owl.DotenvVariable
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		require.True(t, ok, "update line must be KEY=value")
		vars = append(vars, owl.DotenvVariable{Key: key, Value: value, Source: source})
	}
	require.NoError(t, store.ApplyUpdate(ctx, owl.UpdateInput{
		Source: source,
		Dotenv: vars,
		Delete: deleted,
	}))
}

func TestPublicAPIOperationsUseGraphShapedLoadInput(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot, err := store.Snapshot(context.Background(), owl.SnapshotInput{
		Policy: owl.SnapshotPolicy{Reveal: true},
		Filter: owl.SnapshotFilter{Limit: 1},
	})
	require.NoError(t, err)
	require.Len(t, snapshot.Envs, 1)
	assert.Equal(t, "API_URL", snapshot.Envs[0].Name)
	assert.Equal(t, owl.Source{Name: ".env", Kind: "dotenv"}, snapshot.Envs[0].Source)

	source, err := store.Source(context.Background(), owl.SourceInput{
		Policy: owl.DotenvPolicy{Insecure: true},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
	}, source.Envs)

	check, err := store.Check(context.Background(), owl.CheckInput{})
	require.NoError(t, err)
	assert.True(t, check.OK)
	assert.Equal(t, 2, check.Checked)

	for _, op := range []owl.GraphOperation{
		mustBuildSnapshotOperation(t, store),
		mustBuildSourceOperation(t, store),
		mustBuildGetOperation(t, store),
		mustBuildSensitiveKeysOperation(t, store),
		mustBuildDotenvSpecOperation(t, store),
		mustBuildProjectSpecOperation(t, store),
		mustBuildTypeOperation(t, store),
		mustBuildCheckOperation(t, store),
		mustBuildUpdateOperation(t, store),
	} {
		if input, ok := op.Variables["input"].(map[string]interface{}); ok {
			assert.Contains(t, input, "envelope")
		}
		assert.NotEmpty(t, op.Variables)
	}
}

func mustBuildSnapshotOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildSnapshotOperation(context.Background(), owl.SnapshotInput{})
	require.NoError(t, err)
	assert.Equal(t, "OwlSnapshot", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildSourceOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildSourceOperation(context.Background(), owl.SourceInput{})
	require.NoError(t, err)
	assert.Equal(t, "OwlDotenv", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildGetOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildGetOperation(context.Background(), owl.GetInput{Key: "API_KEY"})
	require.NoError(t, err)
	assert.Equal(t, "OwlGet", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildSensitiveKeysOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildSensitiveKeysOperation(context.Background(), owl.SensitiveKeysInput{})
	require.NoError(t, err)
	assert.Equal(t, "OwlSensitiveKeys", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildDotenvSpecOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildDotenvSpecOperation(context.Background(), owl.DotenvSpecInput{})
	require.NoError(t, err)
	assert.Equal(t, "OwlDotenvSpec", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildProjectSpecOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildProjectSpecOperation(context.Background(), owl.ProjectSpecInput{})
	require.NoError(t, err)
	assert.Equal(t, "OwlProjectSpec", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildTypeOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildTypeOperation(context.Background(), owl.TypeInput{Policy: owl.TypePolicy{All: true}})
	require.NoError(t, err)
	assert.Equal(t, "OwlTypeSuggestions", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildCheckOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildCheckOperation(context.Background(), owl.CheckInput{})
	require.NoError(t, err)
	assert.Equal(t, "OwlCheck", op.Name)
	assert.Contains(t, op.Document, "$input: LoadInput!")
	return op
}

func mustBuildUpdateOperation(t *testing.T, store *owl.Store) owl.GraphOperation {
	t.Helper()
	op, err := store.BuildUpdateOperation(context.Background(), owl.UpdateInput{
		Dotenv: []owl.DotenvVariable{{Key: "API_URL", Value: "https://next.example.com"}},
		Delete: []string{"API_KEY"},
	})
	require.NoError(t, err)
	assert.Equal(t, "OwlStateEnvelope", op.Name)
	assert.Contains(t, op.Document, "update")
	assert.Contains(t, op.Document, "delete")
	return op
}

func TestPublicAPISnapshotOrderSurvivesStateEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("OMEGA=value\nAPPLE=value\nZETA=value\nBETA=value\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("ZETA=\"Zeta\" # Plain\nBETA=\"Beta\" # Plain\n")),
	)
	require.NoError(t, err)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)

	roundTripped, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)

	snapshot := snapshotItems(t, roundTripped, owl.SnapshotPolicy{Reveal: true})
	assert.Equal(t, []string{"ZETA", "BETA", "APPLE", "OMEGA"}, snapshotNames(snapshot))
}

func TestPublicAPIVisibilityAndExposure(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	byName := snapshotByName(snapshot)

	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, owl.VisibilityLiteral, byName["API_URL"].Visibility)
	assert.Equal(t, owl.ExposureClear, byName["API_URL"].Exposure)

	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, owl.VisibilityMasked, byName["API_KEY"].Visibility)
	assert.Equal(t, owl.ExposureClear, byName["API_KEY"].Exposure)

	assert.Equal(t, "[hidden]", byName["DATABASE_URL"].Value)
	assert.Equal(t, owl.VisibilityHidden, byName["DATABASE_URL"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, byName["DATABASE_URL"].Exposure)

	assert.Equal(t, "[unset]", byName["MISSING_TOKEN"].Value)
	assert.Empty(t, byName["MISSING_TOKEN"].OriginalValue)
	assert.Equal(t, "Missing token", byName["MISSING_TOKEN"].Description)
	assert.Equal(t, owl.VisibilityUnresolved, byName["MISSING_TOKEN"].Visibility)
	assert.Equal(t, owl.ExposureClear, byName["MISSING_TOKEN"].Exposure)
	assert.Empty(t, byName["MISSING_TOKEN"].Source)
	assert.Equal(t, ".env.spec", byName["MISSING_TOKEN"].Origin.Name)

	revealed := snapshotItems(t, store, owl.SnapshotPolicy{Reveal: true})
	revealedByName := snapshotByName(revealed)
	assert.Equal(t, "secret", revealedByName["API_KEY"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["API_KEY"].Visibility)
	assert.Equal(t, "postgres://example", revealedByName["DATABASE_URL"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["DATABASE_URL"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, revealedByName["DATABASE_URL"].Exposure)
}

func TestPublicAPIUndeclaredOpaqueKeysStayHidden(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv("[process]", strings.NewReader("OPENAI_API_KEY=sk-example\nSOMETHING_TOKEN=token-value\nREDIS_PASSWORD=hunter2\n")),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	byName := snapshotByName(snapshot)

	for _, name := range []string{"OPENAI_API_KEY", "SOMETHING_TOKEN", "REDIS_PASSWORD"} {
		assert.Equal(t, "[hidden]", byName[name].Value)
		assert.Empty(t, byName[name].OriginalValue)
		assert.Equal(t, owl.TypeCoreOpaque, byName[name].Type)
		assert.Equal(t, owl.VisibilityHidden, byName[name].Visibility)
		assert.Equal(t, owl.ExposureOpaque, byName[name].Exposure)
		assert.Equal(t, "[process]", byName[name].Source.Name)
	}

	revealed := snapshotItems(t, store, owl.SnapshotPolicy{Reveal: true})
	revealedByName := snapshotByName(revealed)
	assert.Equal(t, "sk-example", revealedByName["OPENAI_API_KEY"].Value)
	assert.Equal(t, "token-value", revealedByName["SOMETHING_TOKEN"].Value)
	assert.Equal(t, "hunter2", revealedByName["REDIS_PASSWORD"].Value)
}

func TestPublicAPIObservedEmptyValuesArePresent(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv("[process]", strings.NewReader("RUNME_TEST_TOKEN=\nEMPTY_OPAQUE=\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("RUNME_TEST_TOKEN=\"Runme test token\" # Secret\nEMPTY_OPAQUE=\"Opaque value\" # Opaque\n")),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	byName := snapshotByName(snapshot)

	assert.Equal(t, "[masked]", byName["RUNME_TEST_TOKEN"].Value)
	assert.Empty(t, byName["RUNME_TEST_TOKEN"].OriginalValue)
	assert.Equal(t, owl.VisibilityMasked, byName["RUNME_TEST_TOKEN"].Visibility)
	assert.Equal(t, owl.ExposureClear, byName["RUNME_TEST_TOKEN"].Exposure)
	assert.Equal(t, "[process]", byName["RUNME_TEST_TOKEN"].Source.Name)
	assert.Equal(t, ".env.spec", byName["RUNME_TEST_TOKEN"].Origin.Name)

	assert.Equal(t, "[hidden]", byName["EMPTY_OPAQUE"].Value)
	assert.Empty(t, byName["EMPTY_OPAQUE"].OriginalValue)
	assert.Equal(t, owl.VisibilityHidden, byName["EMPTY_OPAQUE"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, byName["EMPTY_OPAQUE"].Exposure)
	assert.Equal(t, "[process]", byName["EMPTY_OPAQUE"].Source.Name)
	assert.Equal(t, ".env.spec", byName["EMPTY_OPAQUE"].Origin.Name)

	revealed := snapshotItems(t, store, owl.SnapshotPolicy{Reveal: true})
	revealedByName := snapshotByName(revealed)
	assert.Equal(t, "", revealedByName["RUNME_TEST_TOKEN"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["RUNME_TEST_TOKEN"].Visibility)
	assert.Equal(t, "", revealedByName["EMPTY_OPAQUE"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["EMPTY_OPAQUE"].Visibility)

	check := checkStore(t, store)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "type.invalid-secret")
	assert.NotContains(t, diagnosticCodes(check.Diagnostics), "dotenv.unresolved-required")
}

func TestPublicAPIGetRevealPolicy(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\n")),
	)
	require.NoError(t, err)

	got, ok, err := store.Get(context.Background(), owl.GetInput{Key: "API_KEY"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[masked]", got.Value)
	assert.Equal(t, owl.VisibilityMasked, got.Visibility)
	assert.Equal(t, owl.ExposureClear, got.Exposure)

	got, ok, err = store.Get(context.Background(), owl.GetInput{Key: "API_KEY", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "secret", got.Value)
	assert.Equal(t, owl.VisibilityLiteral, got.Visibility)

	got, ok, err = store.Get(context.Background(), owl.GetInput{Key: "DATABASE_URL"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[hidden]", got.Value)
	assert.Equal(t, owl.VisibilityHidden, got.Visibility)
	assert.Equal(t, owl.ExposureOpaque, got.Exposure)

	got, ok, err = store.Get(context.Background(), owl.GetInput{Key: "DATABASE_URL", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "postgres://example", got.Value)
	assert.Equal(t, owl.VisibilityLiteral, got.Visibility)
	assert.Equal(t, owl.ExposureOpaque, got.Exposure)
}

func TestPublicAPIDotenvSecureAndInsecure(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
	)
	require.NoError(t, err)

	safe := dotenvLines(t, store, owl.DotenvPolicy{})
	assert.Equal(t, []string{
		"API_KEY=[masked]",
		"API_URL=https://api.example.com",
		"DATABASE_URL=[hidden]",
	}, safe)

	insecure := dotenvLines(t, store, owl.DotenvPolicy{Insecure: true})
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"DATABASE_URL=postgres://example",
	}, insecure)

	check := checkStore(t, store)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "dotenv.unresolved-required")
}

func TestPublicAPIStateEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\n")),
	)
	require.NoError(t, err)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, envelope.State.Values)

	roundTripped, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	roundTrippedSnapshot := snapshotItems(t, roundTripped, owl.SnapshotPolicy{})
	assert.Equal(t, snapshotByName(snapshot)["API_KEY"].Visibility, snapshotByName(roundTrippedSnapshot)["API_KEY"].Visibility)
	assert.Equal(t, snapshotByName(snapshot)["DATABASE_URL"].Exposure, snapshotByName(roundTrippedSnapshot)["DATABASE_URL"].Exposure)

	applyUpdateLines(context.Background(), t, roundTripped, owl.Source{Name: "[override]", Kind: "dotenv"}, []string{"API_URL=https://next.example.com"}, nil)
	got, ok, err := roundTripped.Get(context.Background(), owl.GetInput{Key: "API_URL", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://next.example.com", got.Value)

	applyUpdateLines(context.Background(), t, roundTripped, owl.Source{Name: "[update]", Kind: "dotenv"}, nil, []string{"API_KEY"})
	_, ok, err = roundTripped.Get(context.Background(), owl.GetInput{Key: "API_KEY", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPublicAPIStateEnvelopeRoundTripPreservesResolverAttempts(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\n")),
	)
	require.NoError(t, err)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	startedAt := time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC)
	attempt := owl.ResolverAttempt{
		ID:            "attempt-000001",
		ResolverID:    "core/dotenv",
		FieldRef:      owl.FieldRef{TypeID: owl.TypeCoreSecret, Instance: "default", Field: "api.key"},
		ProjectionKey: "API_KEY",
		Outcome:       owl.ResolverNotFound,
		Message:       "dotenv value was not present",
		Source:        owl.Source{Name: ".env", Kind: "dotenv"},
		StartedAt:     startedAt,
		FinishedAt:    startedAt.Add(time.Second),
	}
	envelope.State.ResolverAttempts = append(envelope.State.ResolverAttempts, attempt)

	roundTripped, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)
	next, err := roundTripped.StateEnvelope(context.Background())
	require.NoError(t, err)

	require.Len(t, next.State.ResolverAttempts, 1)
	gotAttempt := next.State.ResolverAttempts[0]
	assert.Equal(t, attempt.ID, gotAttempt.ID)
	assert.Equal(t, attempt.ResolverID, gotAttempt.ResolverID)
	assert.Equal(t, attempt.FieldRef, gotAttempt.FieldRef)
	assert.Equal(t, attempt.ProjectionKey, gotAttempt.ProjectionKey)
	assert.Equal(t, attempt.Outcome, gotAttempt.Outcome)
	assert.Equal(t, attempt.Message, gotAttempt.Message)
	assert.Equal(t, attempt.Source, gotAttempt.Source)
	assert.Equal(t, attempt.StartedAt, gotAttempt.StartedAt)
	assert.Equal(t, attempt.FinishedAt, gotAttempt.FinishedAt)
	assert.Empty(t, gotAttempt.Diagnostics)

	got, ok, err := roundTripped.Get(context.Background(), owl.GetInput{Key: "API_URL", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://api.example.com", got.Value)
}

func TestPublicAPIStateEnvelopeExposesUnresolvedFrontier(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret!\n")),
	)
	require.NoError(t, err)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)

	require.Len(t, envelope.State.UnresolvedFrontier.Needs, 2)
	byKey := unresolvedNeedsByKey(envelope.State.UnresolvedFrontier.Needs)
	require.Contains(t, byKey, "API_KEY")
	assert.True(t, byKey["API_KEY"].Required)
	assert.True(t, byKey["API_KEY"].Blocking)
	assert.Equal(t, owl.UnresolvedMissing, byKey["API_KEY"].Reason)
	require.Contains(t, byKey, "API_URL")
	assert.False(t, byKey["API_URL"].Required)
	assert.False(t, byKey["API_URL"].Blocking)
	assert.Equal(t, owl.UnresolvedMissing, byKey["API_URL"].Reason)

	roundTripped, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)
	next, err := roundTripped.StateEnvelope(context.Background())
	require.NoError(t, err)
	assert.Equal(t, envelope.State.UnresolvedFrontier, next.State.UnresolvedFrontier)
}

func TestPublicAPIUpdatesMaterializeFromOperationLog(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret\n")),
	)
	require.NoError(t, err)

	applyUpdateLines(context.Background(), t, store, owl.Source{Name: "[update]", Kind: "dotenv"}, []string{"API_URL=https://one.example.com"}, nil)
	applyUpdateLines(context.Background(), t, store, owl.Source{Name: "[update]", Kind: "dotenv"}, []string{"API_URL=https://two.example.com"}, nil)
	got, ok, err := store.Get(context.Background(), owl.GetInput{Key: "API_URL", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://two.example.com", got.Value)

	applyUpdateLines(context.Background(), t, store, owl.Source{Name: "[update]", Kind: "dotenv"}, nil, []string{"API_KEY"})
	_, ok, err = store.Get(context.Background(), owl.GetInput{Key: "API_KEY", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	assert.False(t, ok)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	roundTripped, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)
	got, ok, err = roundTripped.Get(context.Background(), owl.GetInput{Key: "API_URL", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://two.example.com", got.Value)
}

func TestPublicAPIExecutionInfoSetsUpdateSource(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain\n")),
	)
	require.NoError(t, err)

	ctx := owl.ContextWithExecutionInfo(context.Background(), owl.ExecutionInfo{
		KnownID:     "cell-id",
		KnownName:   "cell-name",
		ExecContext: "direnv",
	})
	applyUpdateLines(ctx, t, store, owl.Source{}, []string{
		"API_URL=https://next.example.com",
		"TOKEN=secret",
	}, nil)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{Reveal: true})
	byName := snapshotByName(snapshot)

	assert.Equal(t, "[direnv]", byName["API_URL"].Source.Name)
	assert.Equal(t, "execution", byName["API_URL"].Source.Kind)
	assert.Equal(t, ".env.spec", byName["API_URL"].Origin.Name)
	assert.False(t, byName["API_URL"].UpdatedAt.IsZero())
	assert.Equal(t, "[direnv]", byName["TOKEN"].Source.Name)
	assert.Equal(t, "[direnv]", byName["TOKEN"].Origin.Name)
	assert.False(t, byName["TOKEN"].UpdatedAt.IsZero())
}

func TestPublicAPIWithEnvContractMapsBindings(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("DATABASE_URL=postgres://example\n")),
		owl.WithEnvContract(owl.EnvContract{
			Source:     owl.Source{Name: "package.json", Kind: "package-json"},
			Projection: "dotenv",
			Bindings: []owl.EnvBinding{
				{
					FieldRef:    owl.FieldRef{TypeID: owl.TypeCoreURL, Instance: "primary", Field: "database.url"},
					Key:         "DATABASE_URL",
					Description: "Database URL",
					Source:      owl.Source{Name: "package.json", Kind: "package-json"},
				},
			},
		}),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	item := snapshotByName(snapshot)["DATABASE_URL"]
	assert.Equal(t, "postgres://example", item.Value)
	assert.Equal(t, owl.TypeCoreURL, item.Type)
	assert.Equal(t, `core/url("primary").database.url`, item.Field.String())
	assert.Equal(t, owl.VisibilityLiteral, item.Visibility)
	assert.Equal(t, owl.ExposureClear, item.Exposure)
	assert.Equal(t, "Database URL", item.Description)
	assert.Equal(t, "package.json", item.Origin.Name)
}

func TestPublicAPIWithConfigMapsRedisRequirement(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithConfig(owl.ConfigInput{
			Needs: []owl.NeedInput{
				{
					ID:       "redis.queues",
					Type:     owl.TypeUniverseRedis,
					Instance: "queues",
					Dotenv: &owl.DotenvProjection{
						Fields: []owl.DotenvFieldBinding{
							{Field: "password", Key: "REDIS_AUTH_TOKEN"},
						},
					},
				},
			},
		}),
		owl.WithDotenv(".env", strings.NewReader("QUEUES_REDIS_HOST=localhost\nQUEUES_REDIS_PORT=6379\n")),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	byName := snapshotByName(snapshot)

	assert.Equal(t, "localhost", byName["QUEUES_REDIS_HOST"].Value)
	assert.Equal(t, owl.TypeUniverseRedis, byName["QUEUES_REDIS_HOST"].Type)
	assert.Equal(t, `universe/redis("queues").host`, byName["QUEUES_REDIS_HOST"].Field.String())
	assert.Equal(t, "Redis server hostname", byName["QUEUES_REDIS_HOST"].Description)
	assert.Equal(t, owl.VisibilityLiteral, byName["QUEUES_REDIS_HOST"].Visibility)

	assert.Equal(t, "6379", byName["QUEUES_REDIS_PORT"].Value)
	assert.Equal(t, `universe/redis("queues").port`, byName["QUEUES_REDIS_PORT"].Field.String())
	assert.Equal(t, "Redis server port", byName["QUEUES_REDIS_PORT"].Description)

	assert.Equal(t, "[unset]", byName["REDIS_AUTH_TOKEN"].Value)
	assert.Equal(t, `universe/redis("queues").password`, byName["REDIS_AUTH_TOKEN"].Field.String())
	assert.Equal(t, "Redis password", byName["REDIS_AUTH_TOKEN"].Description)
	assert.Equal(t, owl.VisibilityUnresolved, byName["REDIS_AUTH_TOKEN"].Visibility)
	assert.Equal(t, "[config]", byName["REDIS_AUTH_TOKEN"].Origin.Name)

	dotenvSpec, err := store.DotenvSpec(context.Background(), owl.DotenvSpecInput{})
	require.NoError(t, err)
	assert.Equal(t, strings.Join([]string{
		"# Generated by Owl from Owl config. Do not edit by hand.",
		"",
		`QUEUES_REDIS_HOST="Redis server hostname" # Plain!`,
		`QUEUES_REDIS_PORT="Redis server port"     # Plain!`,
		`REDIS_AUTH_TOKEN="Redis password"         # Secret!`,
		"",
	}, "\n"), dotenvSpec.Rendered)

	check := checkStore(t, store)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "dotenv.unresolved-required")
}

func TestPublicAPIWithConfigValidatesRedisPort(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithConfig(owl.ConfigInput{
			Needs: []owl.NeedInput{
				{
					ID:       "redis.queues",
					Type:     owl.TypeUniverseRedis,
					Instance: "queues",
				},
			},
		}),
		owl.WithDotenv(".env", strings.NewReader("QUEUES_REDIS_HOST=localhost\nQUEUES_REDIS_PORT=not-a-port\nQUEUES_REDIS_PASSWORD=secret\n")),
	)
	require.NoError(t, err)

	check := checkStore(t, store)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "type.invalid-port")

	port, ok, err := store.Get(context.Background(), owl.GetInput{Key: "QUEUES_REDIS_PORT", Policy: owl.GetPolicy{Reveal: true}})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, `universe/redis("queues").port`, port.Field.String())
}

func TestPublicAPIWithConfigValidatesRedisHostRequiredByRedis(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithConfig(owl.ConfigInput{
			Needs: []owl.NeedInput{
				{
					ID:       "redis.queues",
					Type:     owl.TypeUniverseRedis,
					Instance: "queues",
				},
			},
		}),
		owl.WithDotenv(".env", strings.NewReader("QUEUES_REDIS_HOST=\nQUEUES_REDIS_PORT=6379\nQUEUES_REDIS_PASSWORD=secret\n")),
	)
	require.NoError(t, err)

	check := checkStore(t, store)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "type.invalid-host")
}

func TestPublicAPIWithConfigIncludesRedisHostDiagnosticsInSnapshot(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithConfig(owl.ConfigInput{
			Needs: []owl.NeedInput{
				{
					ID:       "redis.queues",
					Type:     owl.TypeUniverseRedis,
					Instance: "queues",
				},
			},
		}),
		owl.WithDotenv(".env", strings.NewReader("QUEUES_REDIS_HOST=\nQUEUES_REDIS_PORT=6379\nQUEUES_REDIS_PASSWORD=secret\n")),
	)
	require.NoError(t, err)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	byName := snapshotByName(snapshot)
	assert.Contains(t, diagnosticCodes(byName["QUEUES_REDIS_HOST"].Diagnostics), "type.invalid-host")
}

func TestPublicAPIResolveReturnsPromptActionsAndAppliesAnswers(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_KEY=\"API key\" # Secret!\n")),
	)
	require.NoError(t, err)

	result, err := store.Resolve(context.Background(), owl.ResolveInput{
		Policy: owl.ResolvePolicy{AllowInteraction: true},
	})
	require.NoError(t, err)
	require.Len(t, result.Actions, 1)
	require.NotNil(t, result.Actions[0].Prompt)
	assert.NotEmpty(t, store.ResolverAttempts())
	require.Len(t, store.UnresolvedFrontier().Needs, 1)
	assert.Equal(t, "interactive", string(result.Actions[0].Type))
	assert.Equal(t, "API_KEY", string(result.Actions[0].Prompt.ProjectionKey))
	assert.Equal(t, "API key", result.Actions[0].Prompt.Label)
	assert.True(t, result.Actions[0].Prompt.Required)
	assert.False(t, result.Actions[0].Prompt.AllowEmpty)
	assert.Equal(t, "sensitive", string(result.Actions[0].Prompt.Sensitivity))

	applied, err := store.ApplyPromptAnswers(context.Background(), []owl.PromptAnswer{{
		NeedID: result.Actions[0].Prompt.NeedID,
		Value:  "secret",
	}})
	require.NoError(t, err)
	require.Len(t, applied.Attempts, 1)
	assert.Equal(t, owl.ResolverResolved, applied.Attempts[0].Outcome)

	snapshot := snapshotItems(t, store, owl.SnapshotPolicy{})
	byName := snapshotByName(snapshot)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, "[interactive]", byName["API_KEY"].Source.Name)
	assert.Empty(t, byName["API_KEY"].Diagnostics)
}

func TestPublicAPIPromptAnswerDomainInvalidKeepsFrontierOpen(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithConfig(owl.ConfigInput{
			Needs: []owl.NeedInput{{
				ID:       "redis.queues",
				Type:     owl.TypeUniverseRedis,
				Instance: "queues",
			}},
		}),
	)
	require.NoError(t, err)

	result, err := store.Resolve(context.Background(), owl.ResolveInput{
		Policy: owl.ResolvePolicy{AllowInteraction: true},
	})
	require.NoError(t, err)
	actionsByKey := promptActionsByKey(result.Actions)
	require.Contains(t, actionsByKey, "QUEUES_REDIS_PORT")

	applied, err := store.ApplyPromptAnswers(context.Background(), []owl.PromptAnswer{{
		NeedID: actionsByKey["QUEUES_REDIS_PORT"].NeedID,
		Value:  "not-a-port",
	}})
	require.NoError(t, err)
	require.Len(t, applied.Attempts, 1)
	assert.Equal(t, owl.ResolverResolved, applied.Attempts[0].Outcome)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	needs := unresolvedNeedsByKey(envelope.State.UnresolvedFrontier.Needs)
	require.Contains(t, needs, "QUEUES_REDIS_PORT")
	assert.Equal(t, owl.UnresolvedInvalid, needs["QUEUES_REDIS_PORT"].Reason)
	assert.Contains(t, diagnosticCodes(envelope.State.Diagnostics), "type.invalid-port")
}

func TestV2PublicAPIDiagnostics(t *testing.T) {
	t.Parallel()

	_, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("DATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("DATABASE_URL=\"Database URL\" # DatabaseUrl!\n")),
	)
	require.Error(t, err)

	diagnostics := owl.Diagnostics(err)
	require.Len(t, diagnostics, 1)
	assert.Equal(t, owl.DiagnosticError, diagnostics[0].Severity)
	assert.Equal(t, "contract.unknown-type", diagnostics[0].Code)
	assert.Equal(t, "DATABASE_URL", diagnostics[0].Key)
}

func TestV2PublicAPICompileSurface(t *testing.T) {
	t.Parallel()

	var _ owl.StoreOption = owl.WithEnvContract(owl.EnvContract{})
	var _ owl.StoreOption = owl.WithEnvContracts(owl.EnvContract{})
	var _ owl.StoreOption = owl.WithConfig(owl.ConfigInput{})
	var _ owl.StoreOption = owl.WithConfigSource("owl.toml", owl.ConfigInput{})
	var _ owl.StoreOption = owl.WithStateEnvelope(owl.StateEnvelope{})
	var _ owl.Visibility = owl.VisibilityLiteral
	var _ owl.Exposure = owl.ExposureClear

	diagnostics := owl.Diagnostics(errors.New("boom"))
	require.Len(t, diagnostics, 1)
	assert.Equal(t, "owl.error", diagnostics[0].Code)
}

func snapshotByName(items []owl.SnapshotItem) map[string]owl.SnapshotItem {
	result := make(map[string]owl.SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}

func snapshotNames(items []owl.SnapshotItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Name)
	}
	return result
}

func diagnosticCodes(diagnostics []owl.Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func unresolvedNeedsByKey(needs []owl.UnresolvedNeed) map[string]owl.UnresolvedNeed {
	result := make(map[string]owl.UnresolvedNeed, len(needs))
	for _, need := range needs {
		result[string(need.ProjectionKey)] = need
	}
	return result
}

func promptActionsByKey(actions []owl.ResolverAction) map[string]owl.PromptAction {
	result := make(map[string]owl.PromptAction, len(actions))
	for _, action := range actions {
		if action.Prompt == nil {
			continue
		}
		result[string(action.Prompt.ProjectionKey)] = *action.Prompt
	}
	return result
}
