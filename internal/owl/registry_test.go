package owl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltInRegistry_TypeProvider(t *testing.T) {
	t.Parallel()

	var provider TypeProvider = NewBuiltInRegistry()

	def, ok := provider.ResolveType(TypeUniverseRedis)
	require.True(t, ok)
	assert.Equal(t, TypeUniverseRedis, def.ID)
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
	assert.Equal(t, TypeUniverseRedis, def.ID)

	def, ok, err = provider.ResolveTypeRef("https://owl.runme.dev/v1/types/core/opaque")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, TypeCoreOpaque, def.ID)

	_, _, err = provider.ResolveTypeRef("universe/Redis")
	require.Error(t, err)

	_, _, err = provider.ResolveTypeRef("https://owl.runme.dev/v1/types/universe/Redis")
	require.Error(t, err)
}
