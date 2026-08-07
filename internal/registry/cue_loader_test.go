package registry

import (
	"os/exec"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue/cuecontext"
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
	assert.Equal(t, model.SensitivityPlaintext, plain.Sensitivity)
	assert.Equal(t, "builtin-cue", plain.Source)
	assert.Empty(t, plain.Fields)

	opaque := types[model.TypeCoreOpaque]
	assert.Equal(t, model.SensitivityUnknown, opaque.Sensitivity)

	secret := types[model.TypeCoreSecret]
	assert.Equal(t, model.SensitivitySensitive, secret.Sensitivity)

	url := types[model.TypeCoreURL]
	assert.Equal(t, model.SensitivityPlaintext, url.Sensitivity)

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
	assert.True(t, redis.Fields["host"].Required)
	assert.True(t, redis.Fields["port"].Required)
	assert.Equal(t, model.SensitivityPlaintext, redis.Fields["host"].Sensitivity)
	assert.Equal(t, model.SensitivityPlaintext, redis.Fields["port"].Sensitivity)
	assert.True(t, redis.Fields["password"].Required)
	assert.Equal(t, model.SensitivitySensitive, redis.Fields["password"].Sensitivity)
	assert.Equal(t, "Redis password.", redis.Fields["password"].Description)

	openai := types[model.TypeUniverseOpenAI]
	assert.Equal(t, model.TypeUniverseOpenAI, openai.ID)
	assert.Equal(t, model.FieldKindObject, openai.Kind)
	assert.Equal(t, "OpenAI API client configuration.", openai.Description)
	require.Contains(t, openai.Fields, "apiKey")
	require.Contains(t, openai.Fields, "baseURL")
	require.Contains(t, openai.Fields, "organization")
	require.Contains(t, openai.Fields, "project")
	assert.True(t, openai.Fields["apiKey"].Required)
	assert.False(t, openai.Fields["baseURL"].Required)
	assert.Equal(t, model.TypeCoreSecret, openai.Fields["apiKey"].TypeID)
	assert.Equal(t, model.TypeCoreURL, openai.Fields["baseURL"].TypeID)
	assert.Equal(t, model.SensitivitySensitive, openai.Fields["apiKey"].Sensitivity)
	assert.Equal(t, model.SensitivityPlaintext, openai.Fields["baseURL"].Sensitivity)

	anthropic := types[model.TypeUniverseAnthropic]
	assert.Equal(t, model.TypeUniverseAnthropic, anthropic.ID)
	assert.Equal(t, model.FieldKindObject, anthropic.Kind)
	assert.Equal(t, "Anthropic API client configuration.", anthropic.Description)
	require.Contains(t, anthropic.Fields, "apiKey")
	require.Contains(t, anthropic.Fields, "baseURL")
	assert.True(t, anthropic.Fields["apiKey"].Required)
	assert.False(t, anthropic.Fields["baseURL"].Required)
	assert.Equal(t, model.TypeCoreSecret, anthropic.Fields["apiKey"].TypeID)
	assert.Equal(t, model.TypeCoreURL, anthropic.Fields["baseURL"].TypeID)
	assert.Equal(t, model.SensitivitySensitive, anthropic.Fields["apiKey"].Sensitivity)
	assert.Equal(t, model.SensitivityPlaintext, anthropic.Fields["baseURL"].Sensitivity)
}

func TestTrimpathBinaryUsesEmbeddedCUECatalog(t *testing.T) {
	root := repoRoot(t)
	binary := filepath.Join(t.TempDir(), "owl")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, filepath.Join(root, "main.go"))
	build.Dir = root
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	run := exec.Command(binary,
		"check",
		"--config", filepath.Join(root, "examples/redis/owl.toml"),
		"--env-file", filepath.Join(root, "cmd/testdata/cli/redis.env"),
	)
	run.Dir = t.TempDir()
	output, err = run.CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "ok:")
	assert.NotContains(t, string(output), "type.invalid-")
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

	def, ok, err = registry.ResolveTypeRef("universe/openai")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseOpenAI, def.ID)

	def, ok, err = registry.ResolveTypeRef("universe/anthropic")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, model.TypeUniverseAnthropic, def.ID)
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

	openaiAPIKey := model.FieldRef{TypeID: model.TypeUniverseOpenAI, Instance: "default", Field: "apiKey"}
	assert.NoError(t, registry.ValidateFieldValue(openaiAPIKey, "sk-test"))
	assert.Error(t, registry.ValidateFieldValue(openaiAPIKey, ""))

	openaiBaseURL := model.FieldRef{TypeID: model.TypeUniverseOpenAI, Instance: "default", Field: "baseURL"}
	assert.NoError(t, registry.ValidateFieldValue(openaiBaseURL, "https://api.openai.com/v1"))
	assert.Error(t, registry.ValidateFieldValue(openaiBaseURL, "api.openai.com/v1"))

	anthropicAPIKey := model.FieldRef{TypeID: model.TypeUniverseAnthropic, Instance: "default", Field: "apiKey"}
	assert.NoError(t, registry.ValidateFieldValue(anthropicAPIKey, "sk-ant-test"))
	assert.Error(t, registry.ValidateFieldValue(anthropicAPIKey, ""))

	anthropicBaseURL := model.FieldRef{TypeID: model.TypeUniverseAnthropic, Instance: "default", Field: "baseURL"}
	assert.NoError(t, registry.ValidateFieldValue(anthropicBaseURL, "https://api.anthropic.com"))
	assert.Error(t, registry.ValidateFieldValue(anthropicBaseURL, "api.anthropic.com"))
}

func TestCUETypeDefRequiresExplicitFieldRequiredness(t *testing.T) {
	t.Parallel()

	value := cuecontext.New().CompileString(`
package broken

#Broken: {
	id:          "github.com/runmedev/owl/types/universe/redis"
	kind:        "composite"
	description: "Broken type."
	fields: {
		host: {
			type:        "github.com/runmedev/owl/types/core/plain"
			description: "Host."
			value:       string
		}
	}
}
`)
	require.NoError(t, value.Err())

	_, err := cueTypeDefFromValue(cueTypeSpec{importPath: "test", definition: "#Broken", name: "broken"}, value, map[model.TypeID]model.TypeDef{
		model.TypeCorePlain: {Sensitivity: model.SensitivityPlaintext},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field host")
	assert.Contains(t, err.Error(), "required")
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	return root
}
