package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandLayeringUsesPublicPackages(t *testing.T) {
	t.Parallel()

	internalImport := `"github.com/runmedev/owl/` + `internal/`
	for _, file := range goFiles(t, ".") {
		raw, err := os.ReadFile(file)
		require.NoError(t, err)
		require.NotContains(t, string(raw), internalImport, file)
	}
}

func TestCommandAndSeedAvoidLegacyStoreHelpers(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		".SnapshotItems(",
		".CheckState(",
		".LoadDotenv(",
		".LoadDotenvLines(",
		"store.Update(",
		"store.Delete(",
		"roundTripped.Update(",
		"roundTripped.Delete(",
	}
	for _, root := range []string{".", "../internal/seed"} {
		for _, file := range goFiles(t, root) {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(file)
			require.NoError(t, err)
			for _, pattern := range forbidden {
				require.NotContains(t, string(raw), pattern, file)
			}
		}
	}
}

func goFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	}))
	return files
}
