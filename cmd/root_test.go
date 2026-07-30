package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandDoesNotRenderUsageOnCheckFailure(t *testing.T) {
	withProcessEnv(t, nil)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "owl.toml")
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(configPath, []byte(`
[needs.redis.queues]
type = "github.com/runmedev/owl/types/universe/redis"

[needs.redis.queues.dotenv]
password = "REDIS_AUTH_TOKEN"
`), 0o600))
	require.NoError(t, os.WriteFile(envPath, []byte(`
QUEUES_REDIS_HOST="123"
QUEUES_REDIS_PORT="abc"
REDIS_AUTH_TOKEN=""
`), 0o600))

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--config", configPath, "--env-file", envPath})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, out.String(), "type.invalid-password")
	assert.Contains(t, out.String(), "type.invalid-port")
	assert.NotContains(t, out.String(), "dotenv.unresolved-required")
	assert.NotContains(t, out.String(), "\n  cue:")
	assert.NotContains(t, out.String(), "Error:")
	assert.NotContains(t, out.String(), "Usage:")
}

func TestRootCommandCheckDetailsRendersCueDetails(t *testing.T) {
	withProcessEnv(t, nil)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "owl.toml")
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(configPath, []byte(`
[needs.redis.queues]
type = "github.com/runmedev/owl/types/universe/redis"

[needs.redis.queues.dotenv]
password = "REDIS_AUTH_TOKEN"
`), 0o600))
	require.NoError(t, os.WriteFile(envPath, []byte(`
QUEUES_REDIS_HOST="not a host"
QUEUES_REDIS_PORT="abc"
REDIS_AUTH_TOKEN="secret"
`), 0o600))

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--details", "--config", configPath, "--env-file", envPath})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, out.String(), "type.invalid-host")
	assert.Contains(t, out.String(), "\n  cue: must be an IP address or DNS hostname")
	assert.Contains(t, out.String(), "\n  cue: must be an integer between 1 and 65535")
	assert.NotContains(t, out.String(), "Usage:")
}

func TestRootCommandExampleGoldenOutputs(t *testing.T) {
	withProcessEnv(t, nil)

	tests := []struct {
		name       string
		args       []string
		goldenPath string
		wantErr    bool
	}{
		{
			name:       "redis project spec",
			args:       []string{"project", "spec", "--config", "../examples/redis/owl.toml"},
			goldenPath: "testdata/cli/project_spec.golden",
		},
		{
			name:       "openai project spec",
			args:       []string{"project", "spec", "--config", "../examples/openai/owl.toml"},
			goldenPath: "testdata/cli/openai_project_spec.golden",
		},
		{
			name:       "anthropic project spec",
			args:       []string{"project", "spec", "--config", "../examples/anthropic/owl.toml"},
			goldenPath: "testdata/cli/anthropic_project_spec.golden",
		},
		{
			name:       "check success",
			args:       []string{"check", "--config", "../examples/redis/owl.toml", "--env-file", "testdata/cli/redis.env"},
			goldenPath: "testdata/cli/check_success.golden",
		},
		{
			name:       "check failure details",
			args:       []string{"check", "--details", "--config", "../examples/redis/owl.toml", "--env-file", "testdata/cli/redis.invalid.env"},
			goldenPath: "testdata/cli/check_failure_details.golden",
			wantErr:    true,
		},
		{
			name:       "snapshot valid",
			args:       []string{"snapshot", "--config", "../examples/redis/owl.toml", "--env-file", "testdata/cli/redis.env", "--all"},
			goldenPath: "testdata/cli/snapshot_valid.golden",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Empty(t, stderr.String())
			assert.Equal(t, readGolden(t, tt.goldenPath), stdout.String())
		})
	}
}

func withProcessEnv(t *testing.T, env []string) {
	t.Helper()

	previous := processEnviron
	processEnviron = func() []string {
		return append([]string{}, env...)
	}
	t.Cleanup(func() {
		processEnviron = previous
	})
}

func readGolden(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(raw)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text
}
