package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
)

func TestStoreSnapshotSourceAndCheck(t *testing.T) {
	t.Parallel()

	s, err := NewStore(
		WithDotenv(".env", strings.NewReader("API_URL=https://api.example.com\nAPI_KEY=secret\nDATABASE_URL=postgres://example\n")),
		WithEnvSpec(".env.example", strings.NewReader("API_URL=\"API URL\" # Plain\nAPI_KEY=\"API key\" # Secret!\nDATABASE_URL=\"Database URL\" # Opaque\nMISSING_TOKEN=\"Missing token\" # Secret!\n")),
	)
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, model.TypeCorePlain, byName["API_URL"].Type)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, model.TypeCoreSecret, byName["API_KEY"].Type)
	assert.Equal(t, "[hidden]", byName["DATABASE_URL"].Value)
	assert.Equal(t, model.TypeCoreOpaque, byName["DATABASE_URL"].Type)
	assert.Equal(t, "[unset]", byName["MISSING_TOKEN"].Value)
	assert.Equal(t, model.VisibilityUnresolved, byName["MISSING_TOKEN"].Visibility)

	source, err := s.Source(SourcePolicy{Insecure: true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
		"DATABASE_URL=postgres://example",
	}, source)

	check := s.Check()
	assert.False(t, check.OK)
	require.NotEmpty(t, check.Diagnostics)
	assert.Equal(t, model.DiagnosticError, check.Diagnostics[len(check.Diagnostics)-1].Severity)
}

func TestStoreWithDotenv(t *testing.T) {
	t.Parallel()

	s, err := NewStore(WithDotenv("[system]", strings.NewReader("REDIS_HOST=localhost\nREDIS_PORT=6379\n")))
	require.NoError(t, err)

	snapshot, err := s.Snapshot(SnapshotPolicy{Reveal: true})
	require.NoError(t, err)
	byName := snapshotByName(snapshot)

	assert.Equal(t, model.TypeUniverseRedis, byName["REDIS_HOST"].Type)
	assert.Equal(t, `universe/redis("default").host`, byName["REDIS_HOST"].Field.String())
}

func snapshotByName(items []SnapshotItem) map[string]SnapshotItem {
	result := make(map[string]SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}
