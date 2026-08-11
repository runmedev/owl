package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/pkg/owl"
	"github.com/runmedev/owl/pkg/owl/seed"
)

func TestCommandCUECatalogPrecedence(t *testing.T) {
	root := copyCommandCUECatalog(t)
	redisType := filepath.Join(root, "types/universe/redis/type.cue")
	raw, err := os.ReadFile(redisType)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), "(uint & >=1 & <=65535)", "6380", 1))
	require.NoError(t, os.WriteFile(redisType, raw, 0o600))

	runCheck := func(t *testing.T) error {
		t.Helper()
		cmd := NewRootCommand()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{
			"check",
			"--config", filepath.Join(commandRepoRoot(t), "examples/redis/owl.toml"),
			"--env-file", filepath.Join(commandRepoRoot(t), "cmd/testdata/cli/redis.env"),
		})
		return cmd.Execute()
	}

	t.Run("unset uses embedded", func(t *testing.T) {
		withLookupEnv(t, func(string) (string, bool) { return "", false })
		require.NoError(t, runCheck(t))
	})

	t.Run("valid directory replaces validation catalog", func(t *testing.T) {
		withLookupEnv(t, func(key string) (string, bool) {
			require.Equal(t, cueRootEnv, key)
			return root, true
		})
		require.Error(t, runCheck(t))
	})

	t.Run("empty is rejected", func(t *testing.T) {
		withLookupEnv(t, func(string) (string, bool) { return "", true })
		err := runCheck(t)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OWL_CUE_ROOT is set but empty")
	})

	t.Run("invalid never falls back", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		withLookupEnv(t, func(string) (string, bool) { return missing, true })
		err := runCheck(t)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load CUE catalog from OWL_CUE_ROOT")
	})
}

func TestCUERootControlVariableIsNotObserved(t *testing.T) {
	t.Parallel()

	options := LocalStoreOptions{
		ProcessEnv: []string{
			"OWL_CUE_ROOT=/developer/catalog",
			"APPLICATION_VALUE=visible",
		},
		DirenvDir: t.TempDir(),
		Direnv:    seed.DirenvDisabled,
	}
	catalog, _, err := seed.BuildCatalog(context.Background(), seedOptions(options))
	require.NoError(t, err)
	keys := make([]string, 0, len(catalog.All()))
	for _, variable := range catalog.All() {
		keys = append(keys, variable.Key)
	}
	assert.Contains(t, keys, "APPLICATION_VALUE")
	assert.NotContains(t, keys, "OWL_CUE_ROOT")
}

func TestProjectSpecReceivesSelectedTypeProvider(t *testing.T) {
	t.Parallel()

	provider := &trackingTypeProvider{TypeProvider: owl.NewBuiltInTypeProvider()}
	client := NewLocalStoreClient(LocalStoreOptions{TypeProvider: provider})
	result, err := client.ProjectSpec(context.Background(), ProjectSpecRequest{
		ConfigPath: filepath.Join(commandRepoRoot(t), "examples/redis/owl.toml"),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Rendered)
	assert.Positive(t, provider.resolveTypeRefs)
}

type trackingTypeProvider struct {
	owl.TypeProvider
	resolveTypeRefs int
}

func (p *trackingTypeProvider) ResolveTypeRef(ref string) (owl.TypeDef, bool, error) {
	p.resolveTypeRefs++
	return p.TypeProvider.ResolveTypeRef(ref)
}

func withLookupEnv(t *testing.T, fn func(string) (string, bool)) {
	t.Helper()
	previous := lookupEnv
	lookupEnv = fn
	t.Cleanup(func() { lookupEnv = previous })
}

func copyCommandCUECatalog(t *testing.T) string {
	t.Helper()

	root := commandRepoRoot(t)
	destination := t.TempDir()
	for _, name := range []string{"cue.mod", "schema", "types"} {
		require.NoError(t, os.CopyFS(filepath.Join(destination, name), os.DirFS(filepath.Join(root, name))))
	}
	return destination
}

func commandRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	return root
}
