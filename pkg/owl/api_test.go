package owl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/pkg/owl"
)

func TestV2PublicAPI(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithEnvBytes(".env", []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nREDIS_PASSWORD=hunter2\n")),
		owl.WithEnvSpecBytes(".env.spec", []byte("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\n")),
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

func TestPublicAPIVisibilityAndExposure(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithEnvBytes(".env", []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpecBytes(".env.spec", []byte("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot, err := store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, owl.VisibilityLiteral, byName["API_URL"].Visibility)
	assert.Equal(t, owl.ExposureKnown, byName["API_URL"].Exposure)

	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, owl.VisibilityMasked, byName["API_KEY"].Visibility)
	assert.Equal(t, owl.ExposureKnown, byName["API_KEY"].Exposure)

	assert.Equal(t, "[hidden]", byName["DATABASE_URL"].Value)
	assert.Equal(t, owl.VisibilityHidden, byName["DATABASE_URL"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, byName["DATABASE_URL"].Exposure)

	assert.Equal(t, "[unset]", byName["MISSING_TOKEN"].Value)
	assert.Equal(t, owl.VisibilityUnresolved, byName["MISSING_TOKEN"].Visibility)
	assert.Equal(t, owl.ExposureKnown, byName["MISSING_TOKEN"].Exposure)

	revealed, err := store.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	revealedByName := snapshotByName(revealed)
	assert.Equal(t, "secret", revealedByName["API_KEY"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["API_KEY"].Visibility)
	assert.Equal(t, "postgres://example", revealedByName["DATABASE_URL"].Value)
	assert.Equal(t, owl.VisibilityLiteral, revealedByName["DATABASE_URL"].Visibility)
	assert.Equal(t, owl.ExposureOpaque, revealedByName["DATABASE_URL"].Exposure)
}

func TestPublicAPIGetRevealPolicy(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithEnvBytes(".env", []byte("API_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpecBytes(".env.spec", []byte("API_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\n")),
	)
	require.NoError(t, err)

	got, ok, err := store.Get("API_KEY", owl.GetPolicy{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "[masked]", got.Value)
	assert.Equal(t, owl.VisibilityMasked, got.Visibility)
	assert.Equal(t, owl.ExposureKnown, got.Exposure)

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
		owl.WithEnvBytes(".env", []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpecBytes(".env.spec", []byte("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
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
		owl.WithEnvBytes(".env", []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		owl.WithEnvSpecBytes(".env.spec", []byte("API_URL=\"API URL\" # Plain!\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\n")),
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

func TestPublicAPIWithEnvContractMapsBindings(t *testing.T) {
	t.Parallel()

	store, err := owl.NewStore(
		owl.WithEnvBytes(".env", []byte("DATABASE_URL=postgres://example\n")),
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
	assert.Equal(t, owl.ExposureKnown, item.Exposure)
	assert.Equal(t, "Database URL", item.Description)
	assert.Equal(t, "package.json", item.Origin.Name)
}

func TestV2PublicAPIDiagnostics(t *testing.T) {
	t.Parallel()

	_, err := owl.NewStore(
		owl.WithEnvBytes(".env", []byte("DATABASE_URL=postgres://example\n")),
		owl.WithEnvSpecBytes(".env.spec", []byte("DATABASE_URL=\"Database URL\" # Url!\n")),
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
	var _ owl.StoreOption = owl.WithStateEnvelope(owl.StateEnvelope{})
	var _ owl.Visibility = owl.VisibilityLiteral
	var _ owl.Exposure = owl.ExposureKnown

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

func diagnosticCodes(diagnostics []owl.Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}
