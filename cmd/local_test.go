package cmd

import (
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

func TestLocalStoreClientUsesV2StoreSemantics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(envFile, []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot.Envs)

	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, envFile, byName["API_URL"].Source)
	assert.Equal(t, "core/plain", byName["API_URL"].Type)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, envFile, byName["API_KEY"].Source)
	assert.Equal(t, "core/secret", byName["API_KEY"].Type)
	assert.Equal(t, "masked", byName["API_KEY"].Visibility)
	assert.Equal(t, "[hidden]", byName["DATABASE_URL"].Value)
	assert.Equal(t, "core/opaque", byName["DATABASE_URL"].Type)
	assert.Equal(t, "hidden", byName["DATABASE_URL"].Visibility)
	assert.Equal(t, "[unset]", byName["MISSING_TOKEN"].Value)
	assert.Contains(t, byName["MISSING_TOKEN"].Visibility, "dotenv.unresolved-required")

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

	resolved, err := client.resolvedStore(context.Background(), false)
	require.NoError(t, err)
	attempts := resolved.ResolverAttempts()
	require.NotEmpty(t, attempts)
	assert.Equal(t, owl.ResolverID("core/dotenv"), attempts[0].ResolverID)
}

func TestLocalStoreClientSnapshotRequiresRevealAndInsecureForPlaintext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(envFile, []byte("API_KEY=secret\nDATABASE_URL=postgres://example\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{},
	})

	revealOnly, err := client.Snapshot(context.Background(), SnapshotRequest{Reveal: true})
	require.NoError(t, err)
	revealOnlyByName := snapshotByName(revealOnly.Envs)
	assert.Equal(t, "[masked]", revealOnlyByName["API_KEY"].Value)
	assert.Equal(t, "[hidden]", revealOnlyByName["DATABASE_URL"].Value)

	insecureOnly, err := client.Snapshot(context.Background(), SnapshotRequest{Insecure: true})
	require.NoError(t, err)
	insecureOnlyByName := snapshotByName(insecureOnly.Envs)
	assert.Equal(t, "[masked]", insecureOnlyByName["API_KEY"].Value)
	assert.Equal(t, "[hidden]", insecureOnlyByName["DATABASE_URL"].Value)

	revealed, err := client.Snapshot(context.Background(), SnapshotRequest{Reveal: true, Insecure: true})
	require.NoError(t, err)
	revealedByName := snapshotByName(revealed.Envs)
	assert.Equal(t, "secret", revealedByName["API_KEY"].Value)
	assert.Equal(t, "postgres://example", revealedByName["DATABASE_URL"].Value)
	assert.Equal(t, "literal", revealedByName["API_KEY"].Visibility)
	assert.Equal(t, "literal", revealedByName["DATABASE_URL"].Visibility)
}

func TestLocalStoreClientSeedsProcessEnvBaseline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_PROCESS_BASELINE=\"Process baseline\" # Plain!\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{"OWL_PROCESS_BASELINE=from-process"},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	assert.Equal(t, "from-process", snapshotByName(snapshot.Envs)["OWL_PROCESS_BASELINE"].Value)

	check, err := client.Check(context.Background(), CheckRequest{})
	require.NoError(t, err)
	assert.True(t, check.OK)
}

func TestLocalStoreClientSnapshotAllIncludesInheritedProcessEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("API_KEY=\"API key\" # Secret!\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		SpecFiles: []string{specFile},
		ProcessEnv: []string{
			"API_KEY=from-process",
			"UNBOUND_PROCESS_ENV=from-process",
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{All: true})
	require.NoError(t, err)
	byName := snapshotByName(snapshot.Envs)

	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, "core/secret", byName["API_KEY"].Type)
	assert.True(t, byName["API_KEY"].Explicit)
	assert.Equal(t, "[process]", byName["API_KEY"].Source)

	assert.Equal(t, "[hidden]", byName["UNBOUND_PROCESS_ENV"].Value)
	assert.Equal(t, "core/opaque", byName["UNBOUND_PROCESS_ENV"].Type)
	assert.False(t, byName["UNBOUND_PROCESS_ENV"].Explicit)
	assert.Equal(t, "[process]", byName["UNBOUND_PROCESS_ENV"].Source)
}

func TestLocalStoreClientConfigSnapshotMasksSensitiveProviderFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "owl.toml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
[needs.anthropic.default]
type = "github.com/runmedev/owl/types/universe/anthropic"
`), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		ConfigPath: configFile,
		ProcessEnv: []string{
			"ANTHROPIC_API_KEY=not-a-real-key",
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)

	env := snapshotByName(snapshot.Envs)["ANTHROPIC_API_KEY"]
	assert.Equal(t, "[masked]", env.Value)
	assert.Equal(t, "masked", env.Visibility)
	assert.Equal(t, "universe/anthropic", env.Type)
}

func TestLocalStoreClientSnapshotUsesDotenvBeforeProcessResolver(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(envFile, []byte("OWL_PROCESS_OVERRIDE=from-file\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_PROCESS_OVERRIDE=\"Process override\" # Plain!\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{"OWL_PROCESS_OVERRIDE=from-process"},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_PROCESS_OVERRIDE"]
	assert.Equal(t, "from-file", env.Value)
	assert.Equal(t, envFile, env.Source)
}

func TestLocalStoreClientSnapshotUsesLaterEnvFilesBeforeEarlierEnvFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	localEnvFile := filepath.Join(dir, ".env.local")
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(envFile, []byte("OWL_LOCAL_OVERRIDE=from-env\n"), 0o600))
	require.NoError(t, os.WriteFile(localEnvFile, []byte("OWL_LOCAL_OVERRIDE=from-env-local\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_LOCAL_OVERRIDE=\"Local override\" # Plain!\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile, localEnvFile},
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{"OWL_LOCAL_OVERRIDE=from-process"},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_LOCAL_OVERRIDE"]
	assert.Equal(t, "from-env-local", env.Value)
	assert.Equal(t, localEnvFile, env.Source)
}

func TestLocalStoreClientSnapshotUsesDirenvBeforeEnvFilesAndProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	localEnvFile := filepath.Join(dir, ".env.local")
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(envFile, []byte("OWL_DIRENV_OVERRIDE=from-env\n"), 0o600))
	require.NoError(t, os.WriteFile(localEnvFile, []byte("OWL_DIRENV_OVERRIDE=from-env-local\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_DIRENV_OVERRIDE=\"Direnv override\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export OWL_DIRENV_OVERRIDE=from-direnv\n"), 0o600))

	var calls int
	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile, localEnvFile},
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{"OWL_DIRENV_OVERRIDE=from-direnv"},
		Direnv:     seed.DirenvEnabledWarn,
		DirenvDir:  dir,
		DirenvRunner: func(_ context.Context, gotDir string) (map[string]string, error) {
			calls++
			assert.Equal(t, dir, gotDir)
			return map[string]string{"OWL_DIRENV_OVERRIDE": "from-direnv"}, nil
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_DIRENV_OVERRIDE"]
	assert.Equal(t, "from-direnv", env.Value)
	assert.Equal(t, ".envrc", env.Source)
	assert.Equal(t, 1, calls)
}

func TestLocalStoreClientConfigSnapshotUsesDirenvBeforeEnvFilesAndProcess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	configFile := filepath.Join(dir, "owl.toml")
	require.NoError(t, os.WriteFile(envFile, []byte("CACHE_REDIS_HOST=from-env\nCACHE_REDIS_PORT=6370\nCACHE_REDIS_TOKEN=from-env-token\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export CACHE_REDIS_HOST=from-direnv\nexport CACHE_REDIS_PORT=6380\nexport CACHE_REDIS_TOKEN=from-direnv-token\n"), 0o600))
	require.NoError(t, os.WriteFile(configFile, []byte(`
[needs.redis.cache]
type = "github.com/runmedev/owl/types/universe/redis"

[needs.redis.cache.dotenv]
host = "CACHE_REDIS_HOST"
port = "CACHE_REDIS_PORT"
password = "CACHE_REDIS_TOKEN"
`), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		ConfigPath: configFile,
		EnvFiles:   []string{envFile},
		ProcessEnv: []string{
			"CACHE_REDIS_HOST=from-direnv",
			"CACHE_REDIS_PORT=6380",
			"CACHE_REDIS_TOKEN=from-direnv-token",
		},
		Direnv:    seed.DirenvEnabledWarn,
		DirenvDir: dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return map[string]string{
				"CACHE_REDIS_HOST":  "from-direnv",
				"CACHE_REDIS_PORT":  "6380",
				"CACHE_REDIS_TOKEN": "from-direnv-token",
			}, nil
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot.Envs)

	assert.Equal(t, "from-direnv", byName["CACHE_REDIS_HOST"].Value)
	assert.Equal(t, ".envrc", byName["CACHE_REDIS_HOST"].Source)
	assert.Equal(t, "6380", byName["CACHE_REDIS_PORT"].Value)
	assert.Equal(t, ".envrc", byName["CACHE_REDIS_PORT"].Source)
	assert.Equal(t, "[masked]", byName["CACHE_REDIS_TOKEN"].Value)
	assert.Equal(t, ".envrc", byName["CACHE_REDIS_TOKEN"].Source)
}

func TestLocalStoreClientSnapshotKeepsObservedValueWhenDirenvDiffers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_DIRENV_MISMATCH=\"Direnv mismatch\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export OWL_DIRENV_MISMATCH=from-direnv\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{"OWL_DIRENV_MISMATCH=from-process"},
		Direnv:     seed.DirenvEnabledWarn,
		DirenvDir:  dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return map[string]string{"OWL_DIRENV_MISMATCH": "from-direnv"}, nil
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_DIRENV_MISMATCH"]
	assert.Equal(t, "from-process", env.Value)
	assert.Equal(t, "[process]", env.Source)

	check, err := client.Check(context.Background(), CheckRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, check.Diagnostics)
	assert.Contains(t, check.Diagnostics[0], "warning direnv.mismatch OWL_DIRENV_MISMATCH")
}

func TestLocalStoreClientSnapshotKeepsUntypedDirenvValuesHidden(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export OWL_DIRENV_UNTYPED=secret-ish\n"), 0o600))
	client := NewLocalStoreClient(LocalStoreOptions{
		ProcessEnv: []string{},
		Direnv:     seed.DirenvEnabledWarn,
		DirenvDir:  dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return map[string]string{"OWL_DIRENV_UNTYPED": "secret-ish"}, nil
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{All: true})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_DIRENV_UNTYPED"]
	assert.Equal(t, "[hidden]", env.Value)
	assert.Equal(t, ".envrc", env.Source)
	assert.Equal(t, "core/opaque", env.Type)
	assert.Equal(t, "hidden", env.Visibility)
}

func TestLocalStoreClientSnapshotMasksContractTypedDirenvSecret(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_DIRENV_TOKEN=\"Direnv token\" # Secret!\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export OWL_DIRENV_TOKEN=secret\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{},
		Direnv:     seed.DirenvEnabledWarn,
		DirenvDir:  dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return map[string]string{"OWL_DIRENV_TOKEN": "secret"}, nil
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_DIRENV_TOKEN"]
	assert.Equal(t, "[masked]", env.Value)
	assert.Equal(t, ".envrc", env.Source)
	assert.Equal(t, "core/secret", env.Type)
	assert.Equal(t, "masked", env.Visibility)
}

func TestLocalStoreClientDirenvWarnRecordsDiagnosticAndContinues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("bad\n"), 0o600))
	client := NewLocalStoreClient(LocalStoreOptions{
		ProcessEnv: []string{},
		Direnv:     seed.DirenvEnabledWarn,
		DirenvDir:  dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return nil, assert.AnError
		},
	})

	check, err := client.Check(context.Background(), CheckRequest{})
	require.NoError(t, err)
	assert.True(t, check.OK)
	require.NotEmpty(t, check.Diagnostics)
	assert.Contains(t, check.Diagnostics[0], "warning direnv.export")
}

func TestLocalStoreClientZeroValueDirenvUsesWarn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.example")
	require.NoError(t, os.WriteFile(specFile, []byte("OWL_DIRENV_DEFAULT=\"Direnv default\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("export OWL_DIRENV_DEFAULT=from-direnv\n"), 0o600))
	client := NewLocalStoreClient(LocalStoreOptions{
		SpecFiles:  []string{specFile},
		ProcessEnv: []string{},
		DirenvDir:  dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return map[string]string{"OWL_DIRENV_DEFAULT": "from-direnv"}, nil
		},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	env := snapshotByName(snapshot.Envs)["OWL_DIRENV_DEFAULT"]
	assert.Equal(t, "from-direnv", env.Value)
	assert.Equal(t, ".envrc", env.Source)
}

func TestLocalStoreClientDirenvErrorFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".envrc"), []byte("bad\n"), 0o600))
	client := NewLocalStoreClient(LocalStoreOptions{
		ProcessEnv: []string{},
		Direnv:     seed.DirenvEnabledError,
		DirenvDir:  dir,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			return nil, assert.AnError
		},
	})

	_, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.Error(t, err)
}

func TestLocalStoreClientDirenvDisabledSkipsRunner(t *testing.T) {
	t.Parallel()

	client := NewLocalStoreClient(LocalStoreOptions{
		ProcessEnv: []string{},
		Direnv:     seed.DirenvDisabled,
		DirenvRunner: func(context.Context, string) (map[string]string, error) {
			t.Fatal("direnv runner should not run when disabled")
			return nil, nil
		},
	})

	_, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
}

func TestLocalStoreClientAutoloadsV1SpecDefaultsInOrder(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, os.WriteFile(".env", []byte(strings.Join([]string{
		"SAMPLE_KEY=from-sample",
		"EXAMPLE_KEY=from-example",
		"SPEC_KEY=from-spec",
		"",
	}, "\n")), 0o600))
	require.NoError(t, os.WriteFile(".env.sample", []byte("SAMPLE_KEY=\"Sample key\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(".env.example", []byte("EXAMPLE_KEY=\"Example key\" # Plain!\n"), 0o600))
	require.NoError(t, os.WriteFile(".env.spec", []byte("SPEC_KEY=\"Spec key\" # Plain!\n"), 0o600))

	files, err := filesOrDefaults(nil, ".env.sample", ".env.example", ".env.spec")
	require.NoError(t, err)
	assert.Equal(t, []string{".env.sample", ".env.example", ".env.spec"}, files)

	client := NewLocalStoreClient(LocalStoreOptions{
		ProcessEnv: []string{},
	})

	snapshot, err := client.Snapshot(context.Background(), SnapshotRequest{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot.Envs)

	assert.Equal(t, "from-sample", byName["SAMPLE_KEY"].Value)
	assert.Equal(t, "Sample key", byName["SAMPLE_KEY"].Description)
	assert.Equal(t, "from-example", byName["EXAMPLE_KEY"].Value)
	assert.Equal(t, "Example key", byName["EXAMPLE_KEY"].Description)
	assert.Equal(t, "from-spec", byName["SPEC_KEY"].Value)
	assert.Equal(t, "Spec key", byName["SPEC_KEY"].Description)
}

func TestLocalStoreClientTypeProposesMissingPrimitiveTypes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.spec")
	require.NoError(t, os.WriteFile(envFile, []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nTARGET_PLATFORM=darwin/arm64\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		ProcessEnv: []string{},
	})

	result, err := client.Type(context.Background(), TypeRequest{SpecPath: specFile, Format: "dotenv-spec"})
	require.NoError(t, err)

	require.Len(t, result.Proposals, 1)
	assert.Equal(t, "API_KEY", result.Proposals[0].Key)
	assert.Equal(t, "core/secret", result.Proposals[0].SuggestedType)
	assert.Contains(t, result.Rendered, "API_KEY=\"Api Key\" # Secret")
	assert.NotContains(t, result.Rendered, "TARGET_PLATFORM")
}

func TestLocalStoreClientTypeAllIncludesDefaultPlainTypes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.spec")
	require.NoError(t, os.WriteFile(envFile, []byte("API_KEY=secret\nTARGET_PLATFORM=darwin/arm64\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		ProcessEnv: []string{},
	})

	result, err := client.Type(context.Background(), TypeRequest{SpecPath: specFile, All: true})
	require.NoError(t, err)

	require.Len(t, result.Proposals, 2)
	assert.Equal(t, "API_KEY", result.Proposals[0].Key)
	assert.Equal(t, "core/secret", result.Proposals[0].SuggestedType)
	assert.Equal(t, "TARGET_PLATFORM", result.Proposals[1].Key)
	assert.Equal(t, "-", result.Proposals[1].SuggestedType)
	assert.Equal(t, "none", result.Proposals[1].Confidence)
	assert.Equal(t, "no primitive type heuristic matched", result.Proposals[1].Reason)
	assert.NotContains(t, result.Rendered, "TARGET_PLATFORM")
}

func TestRenderDotenvSpecTypeProposalsAlignsComments(t *testing.T) {
	t.Parallel()

	rendered := renderDotenvSpecTypeProposals([]owl.TypeProposal{
		{Key: "GITHUB_TOKEN", SuggestedType: owl.TypeCoreSecret, Description: "The GitHub token to use for API requests."},
		{Key: "RUNME_TEST_TOKEN", SuggestedType: owl.TypeCoreSecret, Description: "The Runme test token to use for integration tests."},
		{Key: "TARGET_PLATFORM", SuggestedType: owl.TypeCorePlain, Description: "The target platform to build binary artifacts for.", Required: true},
		{Key: "UNMATCHED", SuggestedType: "", Description: "No suggestion."},
	})

	assert.Equal(t, strings.Join([]string{
		`GITHUB_TOKEN="The GitHub token to use for API requests."              # Secret`,
		`RUNME_TEST_TOKEN="The Runme test token to use for integration tests." # Secret`,
		`TARGET_PLATFORM="The target platform to build binary artifacts for."  # Plain!`,
		"",
	}, "\n"), rendered)
}

func TestLocalStoreClientTypeFixAppendsSpec(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.spec")
	require.NoError(t, os.WriteFile(envFile, []byte("API_KEY=secret\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		ProcessEnv: []string{},
	})

	_, err := client.Type(context.Background(), TypeRequest{SpecPath: specFile, Fix: true})
	require.NoError(t, err)

	raw, err := os.ReadFile(specFile)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "API_URL=\"API URL\" # Plain\n")
	assert.Contains(t, string(raw), "API_KEY=\"Api Key\" # Secret\n")
}

func TestLocalStoreClientTypeOutputDashRendersChangedSpec(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.spec")
	require.NoError(t, os.WriteFile(envFile, []byte("API_KEY=secret\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		ProcessEnv: []string{},
	})

	result, err := client.Type(context.Background(), TypeRequest{SpecPath: specFile, Output: "-"})
	require.NoError(t, err)

	assert.Equal(t, "API_URL=\"API URL\" # Plain\n\nAPI_KEY=\"Api Key\" # Secret\n", result.Rendered)
	raw, err := os.ReadFile(specFile)
	require.NoError(t, err)
	assert.Equal(t, "API_URL=\"API URL\" # Plain", string(raw))
}

func TestMaterializeDotenvSpecTypeProposalsSeparatesBlocks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specFile := filepath.Join(dir, ".env.spec")

	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain"), 0o600))
	materialized, err := materializeDotenvSpecTypeProposals(specFile, "API_KEY=\"Api Key\" # Secret\n")
	require.NoError(t, err)
	assert.Equal(t, "API_URL=\"API URL\" # Plain\n\nAPI_KEY=\"Api Key\" # Secret\n", materialized)

	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\n"), 0o600))
	materialized, err = materializeDotenvSpecTypeProposals(specFile, "API_KEY=\"Api Key\" # Secret\n")
	require.NoError(t, err)
	assert.Equal(t, "API_URL=\"API URL\" # Plain\n\nAPI_KEY=\"Api Key\" # Secret\n", materialized)

	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\n\n"), 0o600))
	materialized, err = materializeDotenvSpecTypeProposals(specFile, "API_KEY=\"Api Key\" # Secret\n")
	require.NoError(t, err)
	assert.Equal(t, "API_URL=\"API URL\" # Plain\n\nAPI_KEY=\"Api Key\" # Secret\n", materialized)
}

func TestLocalStoreClientTypeUsesDefaultMissingSpec(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.spec")
	require.NoError(t, os.WriteFile(envFile, []byte("API_KEY=secret\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles:   []string{envFile},
		ProcessEnv: []string{},
	})

	result, err := client.Type(context.Background(), TypeRequest{
		SpecPath: specFile,
		Fix:      true,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)

	raw, err := os.ReadFile(specFile)
	require.NoError(t, err)
	assert.Equal(t, "API_KEY=\"Api Key\" # Secret\n", string(raw))
}

func snapshotByName(envs []SnapshotEnv) map[string]SnapshotEnv {
	result := make(map[string]SnapshotEnv, len(envs))
	for _, env := range envs {
		result[env.Name] = env
	}
	return result
}
