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

func TestNewStoreSeedsObservedSourceWithCallerProvenance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("KERNEL_ONLY=\"Kernel value\" # Opaque\n"), 0o600))

	result, err := NewStore(context.Background(), Options{
		SpecFiles: []string{specFile},
		Observed: []ObservedSource{
			{
				Source:  owl.Source{Name: "[kernel]", Kind: "runme-kernel"},
				Environ: []string{"KERNEL_ONLY=from-kernel"},
			},
		},
	})
	require.NoError(t, err)

	_, err = result.Store.Resolve(context.Background(), owl.ResolveInput{
		Process: result.Catalog.ProcessResolverInput(),
		Dotenv:  result.Catalog.DotenvResolverInput(),
	})
	require.NoError(t, err)

	items, err := result.Store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	env := snapshotItemByName(items)["KERNEL_ONLY"]
	assert.Equal(t, "[hidden]", env.Value)
	assert.Equal(t, owl.Source{Name: "[kernel]", Kind: "runme-kernel"}, env.Source)
	assert.Equal(t, owl.TypeCoreOpaque, env.Type)
}

func TestNewStoreResolvesDirenvBeforeObservedEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("DIRENV_WINS=\"Direnv wins\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export DIRENV_WINS=from-direnv\n"), 0o600))

	result, err := NewStore(context.Background(), Options{
		SpecFiles: []string{specFile},
		Observed: []ObservedSource{
			{
				Source:  owl.Source{Name: "[kernel]", Kind: "runme-kernel"},
				Environ: []string{"DIRENV_WINS=from-kernel"},
			},
		},
		WorkDir: dir,
		Direnv:  DirenvEnabledWarn,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return map[string]string{"DIRENV_WINS": "from-direnv"}, nil
		},
	})
	require.NoError(t, err)

	_, err = result.Store.Resolve(context.Background(), owl.ResolveInput{
		Process: result.Catalog.ProcessResolverInput(),
		Dotenv:  result.Catalog.DotenvResolverInput(),
	})
	require.NoError(t, err)

	items, err := result.Store.Snapshot(owl.SnapshotPolicy{})
	require.NoError(t, err)
	env := snapshotItemByName(items)["DIRENV_WINS"]
	assert.Equal(t, "from-direnv", env.Value)
	assert.Equal(t, owl.Source{Name: ".envrc", Kind: "direnv"}, env.Source)
}

func TestProcessEnvDotenvQuotesValuesAndSkipsInvalidKeys(t *testing.T) {
	t.Parallel()

	rendered := processEnvDotenv([]string{
		"OWL_SIMPLE=value",
		"OWL_QUOTED=value with spaces\nand newline",
		"BAD-KEY=skipped",
	})

	assert.Contains(t, rendered, "OWL_SIMPLE=\"value\"\n")
	assert.Contains(t, rendered, "OWL_QUOTED=\"value with spaces\\nand newline\"\n")
	assert.NotContains(t, rendered, "BAD-KEY")
}

func snapshotItemByName(items []owl.SnapshotItem) map[string]owl.SnapshotItem {
	result := make(map[string]owl.SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}
