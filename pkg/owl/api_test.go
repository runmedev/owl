package owl_test

import (
	"context"
	"errors"
	"strings"
	"testing"

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

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	assert.Equal(t, "[masked]", snapshotByName(snapshot)["API_KEY"].Value)

	envs, err := store.Dotenv(owl.DotenvPolicy{Insecure: true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"REDIS_PASSWORD=hunter2",
	}, envs)

	got, ok, err := store.Get("API_KEY", owl.GetPolicy{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[masked]", got.Value)

	keys, err := store.SensitiveKeys()
	require.NoError(t, err)
	assert.Equal(t, []string{"API_KEY", "REDIS_PASSWORD"}, keys)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "owl.store.v2", envelope.ModelVersion)

	next, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)
	require.NoError(t, next.LoadDotenvLines("[override]", "API_URL=https://next.example.com"))

	got, ok, err = next.Get("API_URL", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://next.example.com", got.Value)

	require.NoError(t, next.Delete(context.Background(), "API_KEY"))
	_, ok, err = next.Get("API_KEY", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	assert.False(t, ok)
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

	snapshot, err := roundTripped.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"ZETA", "BETA", "APPLE", "OMEGA"}, snapshotNames(snapshot))
}

func TestPublicAPIVisibilityAndExposure(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
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

	revealed, err := store.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	revealedByName := snapshotByName(revealed)
	assert.Equal(t, "secret", revealedByName["API_KEY"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["API_KEY"].Visibility)
	assert.Equal(t, "postgres://example", revealedByName["DATABASE_URL"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["DATABASE_URL"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, revealedByName["DATABASE_URL"].Exposure)
}

func TestPublicAPIEmptySensitiveValuesAreUnresolved(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv("[system]", strings.NewReader("RUNME_TEST_TOKEN=\nEMPTY_OPAQUE=\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("RUNME_TEST_TOKEN=\"Runme test token\" # Secret\nEMPTY_OPAQUE=\"Opaque value\" # Opaque\n")),
	)
	require.NoError(t, err)

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, "[unset]", byName["RUNME_TEST_TOKEN"].Value)
	assert.Empty(t, byName["RUNME_TEST_TOKEN"].OriginalValue)
	assert.Equal(t, owl.VisibilityUnresolved, byName["RUNME_TEST_TOKEN"].Visibility)
	assert.Equal(t, owl.ExposureClear, byName["RUNME_TEST_TOKEN"].Exposure)
	assert.Equal(t, "[system]", byName["RUNME_TEST_TOKEN"].Source.Name)
	assert.Equal(t, ".env.spec", byName["RUNME_TEST_TOKEN"].Origin.Name)

	assert.Equal(t, "[unset]", byName["EMPTY_OPAQUE"].Value)
	assert.Empty(t, byName["EMPTY_OPAQUE"].OriginalValue)
	assert.Equal(t, owl.VisibilityUnresolved, byName["EMPTY_OPAQUE"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, byName["EMPTY_OPAQUE"].Exposure)
	assert.Equal(t, "[system]", byName["EMPTY_OPAQUE"].Source.Name)
	assert.Equal(t, ".env.spec", byName["EMPTY_OPAQUE"].Origin.Name)
}

func TestPublicAPIGetRevealPolicy(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\n")),
	)
	require.NoError(t, err)

	got, ok, err := store.Get("API_KEY", owl.GetPolicy{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[masked]", got.Value)
	assert.Equal(t, owl.VisibilityMasked, got.Visibility)
	assert.Equal(t, owl.ExposureClear, got.Exposure)

	got, ok, err = store.Get("API_KEY", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "secret", got.Value)
	assert.Equal(t, owl.VisibilityLiteral, got.Visibility)

	got, ok, err = store.Get("DATABASE_URL", owl.GetPolicy{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[hidden]", got.Value)
	assert.Equal(t, owl.VisibilityHidden, got.Visibility)
	assert.Equal(t, owl.ExposureOpaque, got.Exposure)

	got, ok, err = store.Get("DATABASE_URL", owl.GetPolicy{Reveal: true})
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

	safe, err := store.Dotenv(owl.DotenvPolicy{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=[masked]",
		"API_URL=https://api.example.com",
		"DATABASE_URL=[hidden]",
	}, safe)

	insecure, err := store.Dotenv(owl.DotenvPolicy{Insecure: true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"DATABASE_URL=postgres://example",
	}, insecure)

	check := store.Check()
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

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	roundTrippedSnapshot, err := roundTripped.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	assert.Equal(t, snapshotByName(snapshot)["API_KEY"].Visibility, snapshotByName(roundTrippedSnapshot)["API_KEY"].Visibility)
	assert.Equal(t, snapshotByName(snapshot)["DATABASE_URL"].Exposure, snapshotByName(roundTrippedSnapshot)["DATABASE_URL"].Exposure)

	require.NoError(t, roundTripped.LoadDotenvLines("[override]", "API_URL=https://next.example.com"))
	got, ok, err := roundTripped.Get("API_URL", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://next.example.com", got.Value)

	require.NoError(t, roundTripped.Delete(context.Background(), "API_KEY"))
	_, ok, err = roundTripped.Get("API_KEY", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPublicAPIUpdatesMaterializeFromOperationLog(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\n")),
		owl.WithEnvSpec(".env.spec", strings.NewReader("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret\n")),
	)
	require.NoError(t, err)

	require.NoError(t, store.Update(context.Background(), []string{"API_URL=https://one.example.com"}, nil))
	require.NoError(t, store.Update(context.Background(), []string{"API_URL=https://two.example.com"}, nil))
	got, ok, err := store.Get("API_URL", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "https://two.example.com", got.Value)

	require.NoError(t, store.Delete(context.Background(), "API_KEY"))
	_, ok, err = store.Get("API_KEY", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	assert.False(t, ok)

	envelope, err := store.StateEnvelope(context.Background())
	require.NoError(t, err)
	roundTripped, err := owl.NewStore(owl.WithStateEnvelope(envelope))
	require.NoError(t, err)
	got, ok, err = roundTripped.Get("API_URL", owl.GetPolicy{Reveal: true})
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
	require.NoError(t, store.Update(ctx, []string{
		"API_URL=https://next.example.com",
		"TOKEN=secret",
	}, nil))

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
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

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
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

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
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

	dotenvSpec, err := store.DotenvSpec()
	require.NoError(t, err)
	assert.Equal(t, strings.Join([]string{
		"# Generated by Owl from Owl config. Do not edit by hand.",
		"",
		`QUEUES_REDIS_HOST="Redis server hostname" # Host!`,
		`QUEUES_REDIS_PORT="Redis server port"     # Port!`,
		`REDIS_AUTH_TOKEN="Redis password"         # Secret!`,
		"",
	}, "\n"), dotenvSpec)

	check := store.Check()
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

	check := store.Check()
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "type.invalid-port")

	port, ok, err := store.Get("QUEUES_REDIS_PORT", owl.GetPolicy{Reveal: true})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, `universe/redis("queues").port`, port.Field.String())
}

func TestPublicAPIWithConfigValidatesRedisHost(t *testing.T) {
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
		owl.WithDotenv(".env", strings.NewReader("QUEUES_REDIS_HOST=not a host\nQUEUES_REDIS_PORT=6379\nQUEUES_REDIS_PASSWORD=secret\n")),
	)
	require.NoError(t, err)

	check := store.Check()
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
		owl.WithDotenv(".env", strings.NewReader("QUEUES_REDIS_HOST=not a host\nQUEUES_REDIS_PORT=6379\nQUEUES_REDIS_PASSWORD=secret\n")),
	)
	require.NoError(t, err)

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)
	assert.Contains(t, diagnosticCodes(byName["QUEUES_REDIS_HOST"].Diagnostics), "type.invalid-host")
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
