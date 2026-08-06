package requirements

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/store"
)

func TestContractsFromConfigInfersRequiredDotenvBindings(t *testing.T) {
	t.Parallel()

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.queues",
				Type:     model.TypeUniverseRedis,
				Instance: "queues",
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	require.Len(t, contracts[0].Bindings, 3)

	byKey := bindingsByKey(contracts[0].Bindings)
	host := byKey["QUEUES_REDIS_HOST"]
	assert.Equal(t, model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "host"}, host.FieldRef)
	assert.Equal(t, "Redis server hostname", host.Description)
	assert.True(t, host.Required)

	password := byKey["QUEUES_REDIS_PASSWORD"]
	assert.Equal(t, model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "password"}, password.FieldRef)
	assert.Equal(t, "Redis password", password.Description)
	assert.True(t, password.Required)
}

func TestContractsFromConfigUsesPreferredKeysForDefaultInstance(t *testing.T) {
	t.Parallel()

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.default",
				Type:     model.TypeUniverseRedis,
				Instance: "default",
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.NoError(t, err)

	byKey := bindingsByKey(contracts[0].Bindings)
	assert.Contains(t, byKey, "REDIS_HOST")
	assert.Contains(t, byKey, "REDIS_PORT")
	assert.Contains(t, byKey, "REDIS_PASSWORD")
}

func TestContractsFromConfigQualifiesDescriptionsForMultipleInstances(t *testing.T) {
	t.Parallel()

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.default",
				Type:     model.TypeUniverseRedis,
				Instance: "default",
			},
			{
				ID:       "redis.queues",
				Type:     model.TypeUniverseRedis,
				Instance: "queues",
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.NoError(t, err)
	require.Len(t, contracts, 2)

	defaultByKey := bindingsByKey(contracts[0].Bindings)
	assert.Equal(t, "Redis server hostname (default)", defaultByKey["REDIS_HOST"].Description)

	queuesByKey := bindingsByKey(contracts[1].Bindings)
	assert.Equal(t, "Redis server hostname (queues)", queuesByKey["QUEUES_REDIS_HOST"].Description)
	assert.Equal(t, "Redis password (queues)", queuesByKey["QUEUES_REDIS_PASSWORD"].Description)
}

func TestContractsFromConfigAppliesDotenvOverrides(t *testing.T) {
	t.Parallel()

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.queues",
				Type:     model.TypeUniverseRedis,
				Instance: "queues",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "password", Key: "REDIS_AUTH_TOKEN"},
					},
				},
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.NoError(t, err)

	byKey := bindingsByKey(contracts[0].Bindings)
	assert.Contains(t, byKey, "QUEUES_REDIS_HOST")
	assert.Contains(t, byKey, "QUEUES_REDIS_PORT")
	assert.Contains(t, byKey, "REDIS_AUTH_TOKEN")
	assert.NotContains(t, byKey, "QUEUES_REDIS_PASSWORD")
}

func TestContractsFromConfigEmitsProviderRequiredAndExplicitOptionalBindings(t *testing.T) {
	t.Parallel()

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "openai.default",
				Type:     model.TypeUniverseOpenAI,
				Instance: "default",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "baseURL", Key: "OPENAI_BASE_URL"},
						{Field: "organization", Key: "OPENAI_ORG_ID"},
						{Field: "project", Key: "OPENAI_PROJECT_ID"},
					},
				},
			},
			{
				ID:       "anthropic.default",
				Type:     model.TypeUniverseAnthropic,
				Instance: "default",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "baseURL", Key: "ANTHROPIC_BASE_URL"},
					},
				},
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.NoError(t, err)
	require.Len(t, contracts, 2)

	openaiByKey := bindingsByKey(contracts[0].Bindings)
	assert.Contains(t, openaiByKey, "OPENAI_API_KEY")
	assert.True(t, openaiByKey["OPENAI_API_KEY"].Required)
	assert.Equal(t, model.SensitivitySensitive, openaiByKey["OPENAI_API_KEY"].Sensitivity)
	assert.Equal(t, model.ExposureClear, openaiByKey["OPENAI_API_KEY"].Exposure)
	assert.Contains(t, openaiByKey, "OPENAI_BASE_URL")
	assert.False(t, openaiByKey["OPENAI_BASE_URL"].Required)
	assert.Equal(t, model.SensitivityPlaintext, openaiByKey["OPENAI_BASE_URL"].Sensitivity)
	assert.Contains(t, openaiByKey, "OPENAI_ORG_ID")
	assert.False(t, openaiByKey["OPENAI_ORG_ID"].Required)
	assert.Contains(t, openaiByKey, "OPENAI_PROJECT_ID")
	assert.False(t, openaiByKey["OPENAI_PROJECT_ID"].Required)

	anthropicByKey := bindingsByKey(contracts[1].Bindings)
	assert.Contains(t, anthropicByKey, "ANTHROPIC_API_KEY")
	assert.True(t, anthropicByKey["ANTHROPIC_API_KEY"].Required)
	assert.Equal(t, model.SensitivitySensitive, anthropicByKey["ANTHROPIC_API_KEY"].Sensitivity)
	assert.Equal(t, model.ExposureClear, anthropicByKey["ANTHROPIC_API_KEY"].Exposure)
	assert.Contains(t, anthropicByKey, "ANTHROPIC_BASE_URL")
	assert.False(t, anthropicByKey["ANTHROPIC_BASE_URL"].Required)
	assert.Equal(t, model.SensitivityPlaintext, anthropicByKey["ANTHROPIC_BASE_URL"].Sensitivity)
}

func TestContractsFromConfigPreservesFixtureProjectionOverrides(t *testing.T) {
	t.Parallel()

	const fixtureTypeID model.TypeID = "test/fixture/service"
	provider := fixtureTypeProvider{
		types: map[model.TypeID]model.TypeDef{
			fixtureTypeID: {
				ID:   fixtureTypeID,
				Name: "fixture",
				Kind: model.FieldKindObject,
				Fields: map[string]model.FieldDef{
					"requiredToken": {
						Name:               "requiredToken",
						TypeID:             model.TypeCoreSecret,
						Required:           true,
						Sensitivity:        model.SensitivitySensitive,
						Exposure:           model.ExposureClear,
						PreferredDotenvKey: "FIXTURE_REQUIRED_TOKEN",
						Description:        "Fixture required token",
					},
					"optionalURL": {
						Name:        "optionalURL",
						TypeID:      model.TypeCoreURL,
						Required:    false,
						Sensitivity: model.SensitivityPlaintext,
						Exposure:    model.ExposureClear,
						Description: "Fixture optional URL",
					},
				},
			},
		},
	}

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "fixture.custom",
				Type:     fixtureTypeID,
				Instance: "custom",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "optionalURL", Key: "CUSTOM_OPTIONAL_URL"},
					},
				},
			},
		},
	}, model.Source{Name: "owl.toml"}, provider)
	require.NoError(t, err)
	require.Len(t, contracts, 1)

	byKey := bindingsByKey(contracts[0].Bindings)
	assert.Contains(t, byKey, "CUSTOM_FIXTURE_REQUIRED_TOKEN")
	assert.True(t, byKey["CUSTOM_FIXTURE_REQUIRED_TOKEN"].Required)
	assert.Equal(t, model.SensitivitySensitive, byKey["CUSTOM_FIXTURE_REQUIRED_TOKEN"].Sensitivity)
	assert.Equal(t, model.ExposureClear, byKey["CUSTOM_FIXTURE_REQUIRED_TOKEN"].Exposure)
	assert.Contains(t, byKey, "CUSTOM_OPTIONAL_URL")
	assert.False(t, byKey["CUSTOM_OPTIONAL_URL"].Required)
	assert.Equal(t, model.SensitivityPlaintext, byKey["CUSTOM_OPTIONAL_URL"].Sensitivity)
}

func TestContractsFromConfigRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.queues",
				Type:     model.TypeUniverseRedis,
				Instance: "queues",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "username", Key: "REDIS_USERNAME"},
					},
				},
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestContractsFromConfigRejectsDuplicateDotenvKeys(t *testing.T) {
	t.Parallel()

	_, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.queues",
				Type:     model.TypeUniverseRedis,
				Instance: "queues",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "host", Key: "REDIS_DUPLICATE"},
						{Field: "port", Key: "REDIS_DUPLICATE"},
					},
				},
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate dotenv key")
}

func TestRenderDotenvSpec(t *testing.T) {
	t.Parallel()

	contracts, err := ContractsFromConfig(model.ConfigInput{
		Needs: []model.NeedInput{
			{
				ID:       "redis.queues",
				Type:     model.TypeUniverseRedis,
				Instance: "queues",
				Dotenv: &model.DotenvProjectionInput{
					Fields: []model.DotenvFieldBindingInput{
						{Field: "password", Key: "REDIS_AUTH_TOKEN"},
					},
				},
			},
		},
	}, model.Source{Name: "owl.toml"}, registry.NewBuiltInRegistry())
	require.NoError(t, err)

	rendered, err := RenderDotenvSpec(contracts, registry.NewBuiltInRegistry())
	require.NoError(t, err)

	assert.Equal(t, strings.Join([]string{
		"# Generated by Owl from owl.toml. Do not edit by hand.",
		"",
		`QUEUES_REDIS_HOST="Redis server hostname" # Plain!`,
		`QUEUES_REDIS_PORT="Redis server port"     # Plain!`,
		`REDIS_AUTH_TOKEN="Redis password"         # Secret!`,
		"",
	}, "\n"), rendered)
}

func bindingsByKey(bindings []store.EnvBinding) map[string]store.EnvBinding {
	result := make(map[string]store.EnvBinding, len(bindings))
	for _, binding := range bindings {
		result[binding.Key] = binding
	}
	return result
}

type fixtureTypeProvider struct {
	types map[model.TypeID]model.TypeDef
}

func (p fixtureTypeProvider) ResolveType(id model.TypeID) (model.TypeDef, bool) {
	def, ok := p.types[id]
	return def, ok
}

func (p fixtureTypeProvider) ResolveTypeRef(ref string) (model.TypeDef, bool, error) {
	def, ok := p.types[model.TypeID(ref)]
	return def, ok, nil
}
