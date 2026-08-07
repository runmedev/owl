package seed

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
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

func TestNewStoreAttributesMatchingObservedEnvToDirenv(t *testing.T) {
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
				Environ: []string{"DIRENV_WINS=from-direnv"},
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

	dotenvVars := result.Catalog.DotenvResolverInput()
	require.Len(t, dotenvVars, 1)
	assert.Equal(t, "PRESENT", dotenvVars[0].Key)
	assert.Equal(t, "from-dotenv", dotenvVars[0].Value)
	assert.Equal(t, owl.Source{Name: envFile, Kind: "dotenv"}, dotenvVars[0].Source)
}

func TestNewStoreUsesDefaultEnvFilesInPrecedenceOrder(t *testing.T) {
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

	dotenvVars := result.Catalog.DotenvResolverInput()
	require.Len(t, dotenvVars, 4)
	assert.Equal(t, "from-env-dev", dotenvVars[0].Value)
	assert.Equal(t, ".env.dev", filepath.Base(dotenvVars[0].Source.Name))
	assert.Equal(t, "from-env-development", dotenvVars[1].Value)
	assert.Equal(t, ".env.development", filepath.Base(dotenvVars[1].Source.Name))
	assert.Equal(t, "from-env-local", dotenvVars[2].Value)
	assert.Equal(t, ".env.local", filepath.Base(dotenvVars[2].Source.Name))
	assert.Equal(t, "from-env", dotenvVars[3].Value)
	assert.Equal(t, ".env", filepath.Base(dotenvVars[3].Source.Name))
}

func TestNewRawValueStoreSkipsMissingEnvFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("PRESENT=from-dotenv\n"), 0o600))

	store, err := NewRawValueStore(Options{
		WorkDir:  dir,
		EnvFiles: []string{".env.local", ".env"},
	}, false)
	require.NoError(t, err)

	items, err := store.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	env := snapshotItemByName(items)["PRESENT"]
	assert.Equal(t, "from-dotenv", env.Value)
	assert.Equal(t, owl.VisibilityLiteral, env.Visibility)
	assert.Equal(t, "dotenv", env.Source.Kind)
	assert.Equal(t, ".env", filepath.Base(env.Source.Name))
}

func TestNewRawValueStoreUsesDefaultEnvFilesInOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("DEFAULT_ORDER=from-env\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.local"), []byte("DEFAULT_ORDER=from-env-local\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.development"), []byte("DEFAULT_ORDER=from-env-development\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.dev"), []byte("DEFAULT_ORDER=from-env-dev\n"), 0o600))

	store, err := NewRawValueStore(Options{WorkDir: dir}, false)
	require.NoError(t, err)

	items, err := store.Snapshot(owl.SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	env := snapshotItemByName(items)["DEFAULT_ORDER"]
	assert.Equal(t, "from-env-dev", env.Value)
	assert.Equal(t, ".env.dev", filepath.Base(env.Source.Name))
}

func TestDotenvVariablesReturnsEnvFileReadErrorsExceptMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notFile := filepath.Join(dir, "not-a-file")
	require.NoError(t, os.Mkdir(notFile, 0o700))

	_, err := dotenvVariables(Options{
		WorkDir:  dir,
		EnvFiles: []string{".env.local", "not-a-file"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-a-file")
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

func TestStoreBuildersUseConfiguredTypeProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	envFile := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(specFile, []byte("SERVICE_URL=\"Service URL\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(envFile, []byte("SERVICE_URL=https://example.com\n"), 0o600))

	seededProvider := &seedTrackingTypeProvider{BuiltInRegistry: registry.NewBuiltInRegistry()}
	result, err := NewStore(context.Background(), Options{
		SpecFiles:    []string{specFile},
		Observed:     []ObservedSource{{Environ: []string{"SERVICE_URL=https://example.com"}}},
		TypeProvider: seededProvider,
	})
	require.NoError(t, err)
	_, err = result.Store.Resolve(context.Background(), owl.ResolveInput{Process: result.Catalog.ProcessResolverInput()})
	require.NoError(t, err)
	_ = result.Store.Check()
	assert.Positive(t, seededProvider.validations)

	rawProvider := &seedTrackingTypeProvider{BuiltInRegistry: registry.NewBuiltInRegistry()}
	rawStore, err := NewRawValueStore(Options{
		EnvFiles:     []string{envFile},
		SpecFiles:    []string{specFile},
		TypeProvider: rawProvider,
	}, false)
	require.NoError(t, err)
	_ = rawStore.Check()
	assert.Positive(t, rawProvider.validations)
}

func TestRunDirenvExportJSONParsesStdoutWhenDirenvLogsToStderr(t *testing.T) {
	binDir := t.TempDir()
	writeFakeDirenv(t, binDir, `#!/bin/sh
echo "direnv: loading .envrc" >&2
echo '{"CACHE_REDIS_HOST":"from-direnv"}'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	values, err := RunDirenvExportJSON(context.Background(), t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "from-direnv", values["CACHE_REDIS_HOST"])
}

func TestRunDirenvExportJSONParsesJSONAfterStdoutLogs(t *testing.T) {
	binDir := t.TempDir()
	writeFakeDirenv(t, binDir, `#!/bin/sh
echo "direnv: loading .envrc"
echo "direnv: export +CACHE_REDIS_HOST"
echo '{"CACHE_REDIS_HOST":"from-direnv"}'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	values, err := RunDirenvExportJSON(context.Background(), t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "from-direnv", values["CACHE_REDIS_HOST"])
}

func TestRunDirenvExportJSONUnsetsKnownProjectionKeys(t *testing.T) {
	binDir := t.TempDir()
	writeFakeDirenv(t, binDir, `#!/bin/sh
if [ -n "$CACHE_REDIS_HOST" ]; then
  echo '{}'
else
  echo '{"CACHE_REDIS_HOST":"from-direnv"}'
fi
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CACHE_REDIS_HOST", "from-process")

	values, err := runDirenvExportJSON(context.Background(), t.TempDir(), map[string]struct{}{"CACHE_REDIS_HOST": {}})
	require.NoError(t, err)
	assert.Equal(t, "from-direnv", values["CACHE_REDIS_HOST"])
}

func TestRunDirenvExportJSONClearsDirenvState(t *testing.T) {
	binDir := t.TempDir()
	writeFakeDirenv(t, binDir, `#!/bin/sh
if [ -n "$DIRENV_DIFF" ] || [ -n "$DIRENV_DIR" ]; then
  echo '{}'
else
  echo '{"CACHE_REDIS_HOST":"from-direnv"}'
fi
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DIRENV_DIFF", "already-applied")
	t.Setenv("DIRENV_DIR", "-/project")

	values, err := runDirenvExportJSON(context.Background(), t.TempDir(), map[string]struct{}{"CACHE_REDIS_HOST": {}})
	require.NoError(t, err)
	assert.Equal(t, "from-direnv", values["CACHE_REDIS_HOST"])
}

func TestRunDirenvExportJSONTreatsBlockedEnvrcAsError(t *testing.T) {
	binDir := t.TempDir()
	writeFakeDirenv(t, binDir, `#!/bin/sh
echo "direnv: error /tmp/.envrc is blocked" >&2
echo '{"DIRENV_DIFF":"x"}'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := RunDirenvExportJSON(context.Background(), t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".envrc is blocked")
}

func writeFakeDirenv(t *testing.T, dir string, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake direnv fixture is a POSIX #!/bin/sh executable")
	}

	path := filepath.Join(dir, "direnv")
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(script)+"\n"), 0o700))
}

type seedTrackingTypeProvider struct {
	registry.BuiltInRegistry
	validations int
}

func (p *seedTrackingTypeProvider) ValidateValue(id model.TypeID, value string) error {
	p.validations++
	return p.BuiltInRegistry.ValidateValue(id, value)
}

func (p *seedTrackingTypeProvider) ValidateFieldValue(ref model.FieldRef, value string) error {
	p.validations++
	return p.BuiltInRegistry.ValidateFieldValue(ref, value)
}

func snapshotItemByName(items []owl.SnapshotItem) map[string]owl.SnapshotItem {
	result := make(map[string]owl.SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}
