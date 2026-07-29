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
}

func TestLocalStoreClientTypeProposesMissingPrimitiveTypes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	specFile := filepath.Join(dir, ".env.spec")
	require.NoError(t, os.WriteFile(envFile, []byte("API_URL=https://api.example.com\nAPI_KEY=secret\nTARGET_PLATFORM=darwin/arm64\n"), 0o600))
	require.NoError(t, os.WriteFile(specFile, []byte("API_URL=\"API URL\" # Plain\n"), 0o600))

	client := NewLocalStoreClient(LocalStoreOptions{
		EnvFiles: []string{envFile},
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
		EnvFiles: []string{envFile},
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
		EnvFiles: []string{envFile},
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
		EnvFiles: []string{envFile},
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
		EnvFiles: []string{envFile},
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
