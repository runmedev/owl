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
