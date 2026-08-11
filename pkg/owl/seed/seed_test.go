package seed

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/pkg/owl"
)

func snapshotItems(t *testing.T, store *owl.Store, policy owl.SnapshotPolicy) []owl.SnapshotItem {
	t.Helper()
	output, err := store.Snapshot(context.Background(), owl.SnapshotInput{
		Policy: policy,
		Filter: owl.SnapshotFilter{All: true},
	})
	require.NoError(t, err)
	return output.Envs
}

func TestNewStoreReturnsSeededStore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("API_KEY=\"API key\" # Secret!\n"), 0o600))

	result, err := NewStore(context.Background(), Options{
		SpecFiles: []string{specFile},
		Observed: []ObservedSource{
			{Source: owl.Source{Name: "[process]", Kind: "process"}, Environ: []string{"API_KEY=secret"}},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Diagnostics)

	items := snapshotItems(t, result.Store, owl.SnapshotPolicy{Reveal: true})
	require.Len(t, items, 1)
	assert.Equal(t, "API_KEY", items[0].Name)
	assert.Equal(t, "secret", items[0].Value)
}

func TestNewStoreSkipsMissingEnvFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envFile, []byte("PRESENT=from-dotenv\n"), 0o600))

	result, err := NewStore(context.Background(), Options{
		WorkDir:  dir,
		EnvFiles: []string{".env.local", ".env"},
		Observed: []ObservedSource{
			{Source: owl.Source{Name: "[process]", Kind: "process"}},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Diagnostics)

	items := snapshotItems(t, result.Store, owl.SnapshotPolicy{Reveal: true})
	require.Len(t, items, 1)
	assert.Equal(t, "PRESENT", items[0].Name)
	assert.Equal(t, "from-dotenv", items[0].Value)
	assert.Equal(t, "dotenv", items[0].Source.Kind)
	assert.Equal(t, ".env", items[0].Source.Name)
}

func TestNewStoreUsesDefaultEnvFilesInOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("DEFAULT_ORDER=from-env\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.local"), []byte("DEFAULT_ORDER=from-env-local\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.development"), []byte("DEFAULT_ORDER=from-env-development\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.dev"), []byte("DEFAULT_ORDER=from-env-dev\n"), 0o600))

	result, err := NewStore(context.Background(), Options{
		WorkDir: dir,
		Observed: []ObservedSource{
			{Source: owl.Source{Name: "[process]", Kind: "process"}},
		},
	})
	require.NoError(t, err)
	require.Empty(t, result.Diagnostics)

	items := snapshotItems(t, result.Store, owl.SnapshotPolicy{Reveal: true})
	require.Len(t, items, 1)
	assert.Equal(t, "DEFAULT_ORDER", items[0].Name)
	assert.Equal(t, "from-env-dev", items[0].Value)
	assert.Equal(t, ".env.dev", items[0].Source.Name)
}
