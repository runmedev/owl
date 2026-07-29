package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandDoesNotRenderUsageOnCheckFailure(t *testing.T) {
	t.Parallel()

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
	assert.Contains(t, out.String(), "dotenv.unresolved-required")
	assert.Contains(t, out.String(), "type.invalid-port")
	assert.NotContains(t, out.String(), "\n  cue:")
	assert.NotContains(t, out.String(), "Error:")
	assert.NotContains(t, out.String(), "Usage:")
}

func TestRootCommandCheckDetailsRendersCueDetails(t *testing.T) {
	t.Parallel()

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
	assert.Contains(t, out.String(), "\n  cue: invalid value \"not a host\"")
	assert.Contains(t, out.String(), "\n  cue: conflicting values uint and \"abc\"")
	assert.NotContains(t, out.String(), "Usage:")
}
