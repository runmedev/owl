package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStoreClientUsesV2StoreSemantics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(envFile, []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:  []string{envFile},
		SpecFiles: []string{specFile},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot.Envs)

	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, "core/plain", byName["API_URL"].Type)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, "core/secret", byName["API_KEY"].Type)
	assert.Equal(t, "[hidden]", byName["DATABASE_URL"].Value)
	assert.Equal(t, "core/opaque", byName["DATABASE_URL"].Type)
	assert.Equal(t, "[unset]", byName["MISSING_TOKEN"].Value)

	source, err := client.Source(context.Background(), SourceRequest{Insecure: true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"DATABASE_URL=postgres://example",
	}, source.Envs)

	check, err := client.Check(context.Background(), CheckRequest{})
	require.NoError(t, err)
	assert.False(t, check.OK)
	require.NotEmpty(t, check.Diagnostics)
	assert.Contains(t, check.Diagnostics[len(check.Diagnostics)-1], "error dotenv.unresolved-required MISSING_TOKEN")
}

func snapshotByName(envs []SnapshotEnv) map[string]SnapshotEnv {
	result := make(map[string]SnapshotEnv, len(envs))
	for _, env := range envs {
		result[env.Name] = env
	}
	return result
}
