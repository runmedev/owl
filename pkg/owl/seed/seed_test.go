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

	items, err := result.Store.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "API_KEY", items[0].Name)
	assert.Equal(t, "secret", items[0].Value)
}
