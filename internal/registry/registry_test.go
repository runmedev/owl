package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
)

func TestBuiltInRegistry_TypeProvider(t *testing.T) {
	t.Parallel()

	var provider TypeProvider = NewBuiltInRegistry()

	def, ok := provider.ResolveType(model.TypeUniverseRedis)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseRedis, def.ID)
	assert.Equal(t, "builtin-go", def.Source)
	assert.Contains(t, def.Fields, "host")
	assert.Contains(t, def.Fields, "port")
	assert.Contains(t, def.Fields, "password")
}

func TestBuiltInRegistry_ResolveTypeRef(t *testing.T) {
	t.Parallel()

	provider := NewBuiltInRegistry()

	def, ok, err := provider.ResolveTypeRef("universe/redis")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseRedis, def.ID)

	def, ok, err = provider.ResolveTypeRef("https://owl.runme.dev/v1/types/core/opaque")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeCoreOpaque, def.ID)

	def, ok, err = provider.ResolveTypeRef("core/plain")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeCorePlain, def.ID)

	_, _, err = provider.ResolveTypeRef("universe/Redis")
	require.Error(t, err)

	_, _, err = provider.ResolveTypeRef("https://owl.runme.dev/v1/types/universe/Redis")
	require.Error(t, err)
}
