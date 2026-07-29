package registry

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
)

func TestLoadBuiltInCUETypeDefs(t *testing.T) {
	t.Parallel()

	types, err := LoadBuiltInCUETypeDefs(repoRoot(t))
	require.NoError(t, err)

	plain := types[model.TypeCorePlain]
	assert.Equal(t, model.TypeCorePlain, plain.ID)
	assert.Equal(t, model.FieldKindScalar, plain.Kind)
	assert.Equal(t, "builtin-cue", plain.Source)
	assert.Empty(t, plain.Fields)

	redis := types[model.TypeUniverseRedis]
	assert.Equal(t, model.TypeUniverseRedis, redis.ID)
	assert.Equal(t, model.FieldKindObject, redis.Kind)
	assert.Equal(t, "Redis connection configuration.", redis.Description)
	require.Contains(t, redis.Fields, "host")
	require.Contains(t, redis.Fields, "port")
	require.Contains(t, redis.Fields, "password")
	assert.Equal(t, model.TypeCorePlain, redis.Fields["host"].TypeID)
	assert.Equal(t, model.TypeCorePlain, redis.Fields["port"].TypeID)
	assert.Equal(t, model.TypeCoreSecret, redis.Fields["password"].TypeID)
	assert.True(t, redis.Fields["password"].Required)
	assert.Equal(t, model.SensitivitySensitive, redis.Fields["password"].Sensitivity)
	assert.Equal(t, "Redis password.", redis.Fields["password"].Description)
}

func TestNewBuiltInCUERegistry(t *testing.T) {
	t.Parallel()

	registry, err := NewBuiltInCUERegistry(repoRoot(t))
	require.NoError(t, err)

	def, ok := registry.ResolveType(model.TypeUniverseRedis)
	require.True(t, ok)
	assert.Equal(t, "builtin-cue", def.Source)

	def, ok, err = registry.ResolveTypeRef("universe/redis")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseRedis, def.ID)
}

func TestBuiltInCUERegistryValidatesValues(t *testing.T) {
	t.Parallel()

	registry, err := NewBuiltInCUERegistry(repoRoot(t))
	require.NoError(t, err)

	redisHost := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "host"}
	assert.NoError(t, registry.ValidateFieldValue(redisHost, "redis.internal"))
	assert.NoError(t, registry.ValidateFieldValue(redisHost, "127.0.0.1"))
	assert.Error(t, registry.ValidateFieldValue(redisHost, "not a host"))
	assert.Error(t, registry.ValidateFieldValue(redisHost, ""))

	redisPort := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "port"}
	assert.NoError(t, registry.ValidateFieldValue(redisPort, "6379"))
	assert.Error(t, registry.ValidateFieldValue(redisPort, "abc"))
	assert.Error(t, registry.ValidateFieldValue(redisPort, "70000"))
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	return root
}
