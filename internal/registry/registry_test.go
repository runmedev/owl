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

	def, ok = provider.ResolveType(model.TypeUniverseOpenAI)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseOpenAI, def.ID)
	assert.Contains(t, def.Fields, "apiKey")
	assert.Contains(t, def.Fields, "baseURL")
	assert.Contains(t, def.Fields, "organization")
	assert.Contains(t, def.Fields, "project")
	assert.True(t, def.Fields["apiKey"].Required)
	assert.False(t, def.Fields["baseURL"].Required)
	assert.Equal(t, model.SensitivitySensitive, def.Fields["apiKey"].Sensitivity)
	assert.Equal(t, model.SensitivityPlaintext, def.Fields["baseURL"].Sensitivity)

	def, ok = provider.ResolveType(model.TypeUniverseAnthropic)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseAnthropic, def.ID)
	assert.Contains(t, def.Fields, "apiKey")
	assert.Contains(t, def.Fields, "baseURL")
	assert.True(t, def.Fields["apiKey"].Required)
	assert.False(t, def.Fields["baseURL"].Required)
	assert.Equal(t, model.SensitivitySensitive, def.Fields["apiKey"].Sensitivity)
	assert.Equal(t, model.SensitivityPlaintext, def.Fields["baseURL"].Sensitivity)
}

func TestBuiltInRegistry_ResolveTypeRef(t *testing.T) {
	t.Parallel()

	provider := NewBuiltInRegistry()

	def, ok, err := provider.ResolveTypeRef("universe/redis")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseRedis, def.ID)

	def, ok, err = provider.ResolveTypeRef("github.com/runmedev/owl/types/core/opaque")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeCoreOpaque, def.ID)

	def, ok, err = provider.ResolveTypeRef("core/plain")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeCorePlain, def.ID)

	def, ok, err = provider.ResolveTypeRef("universe/openai")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseOpenAI, def.ID)

	def, ok, err = provider.ResolveTypeRef("universe/anthropic")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseAnthropic, def.ID)

	_, _, err = provider.ResolveTypeRef("universe/Redis")
	require.Error(t, err)

	_, _, err = provider.ResolveTypeRef("https://owl.runme.dev/v1/types/universe/Redis")
	require.Error(t, err)
}
