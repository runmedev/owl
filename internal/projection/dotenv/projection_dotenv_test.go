package dotenv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
)

func TestIngestDotenv_RedisAndOpaque(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	state := IngestDotenv(map[string]string{
		"REDIS_HOST":            "localhost",
		"REDIS_PORT":            "6379",
		"REDIS_PASSWORD":        "secret-redis",
		"QUEUES_REDIS_HOST":     "queue.local",
		"QUEUES_REDIS_PORT":     "6380",
		"DATABASE_URL":          "postgres://example",
		"CUSTOM_SERVICE_TOKEN":  "token-value",
		"CUSTOM_VISIBLE_STRING": "hello",
	}, DotenvIngestOptions{
		Source:       model.Source{Name: ".env.local", Kind: "dotenv"},
		Actor:        "test",
		Clock:        func() time.Time { return now },
		OperationIDs: model.NewMonotonicOperationIDGenerator("test-op"),
	})

	require.Len(t, state.Values, 8)
	require.Len(t, state.Bindings, 8)

	defaultHost := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "default", Field: "host"}
	require.Contains(t, state.Values, defaultHost)
	assert.Equal(t, "localhost", state.Values[defaultHost].Resolved)
	assert.Equal(t, model.SensitivityNonSensitive, state.Values[defaultHost].Sensitivity)
	assert.Equal(t, model.ExposureClear, state.Values[defaultHost].Exposure)
	assert.Equal(t, model.OperationID("test-op-000006"), state.Values[defaultHost].LastOperationID)

	queuesPort := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "port"}
	require.Contains(t, state.Values, queuesPort)
	assert.Equal(t, "6380", state.Values[queuesPort].Resolved)

	password := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "default", Field: "password"}
	require.Contains(t, state.Values, password)
	assert.Equal(t, model.SensitivitySensitive, state.Values[password].Sensitivity)

	databaseURL := model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: "database.url"}
	require.Contains(t, state.Values, databaseURL)
	assert.Equal(t, model.TypeCoreOpaque, state.Values[databaseURL].FieldRef.TypeID)
	assert.Equal(t, model.SensitivityUnknown, state.Values[databaseURL].Sensitivity)
	assert.Equal(t, model.ExposureOpaque, state.Values[databaseURL].Exposure)

	token := model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: "custom.service.token"}
	require.Contains(t, state.Values, token)
	assert.Equal(t, model.SensitivitySensitive, state.Values[token].Sensitivity)

	require.NotEmpty(t, state.Diagnostics)
	assert.Contains(t, diagnosticCodes(state.Diagnostics), "dotenv.opaque")
}

func TestRenderDotenv_SafeAndInsecure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	state := IngestDotenv(map[string]string{
		"REDIS_HOST":           "localhost",
		"REDIS_PASSWORD":       "secret-redis",
		"DATABASE_URL":         "postgres://example",
		"CUSTOM_SERVICE_TOKEN": "token-value",
	}, DotenvIngestOptions{
		Source:       model.Source{Name: ".env", Kind: "dotenv"},
		Clock:        func() time.Time { return now },
		OperationIDs: model.NewMonotonicOperationIDGenerator("render-op"),
	})

	safe := renderedByKey(RenderDotenv(state, model.RenderPolicy{}))
	assert.Equal(t, "localhost", safe["REDIS_HOST"].Value)
	assert.Equal(t, "[masked]", safe["REDIS_PASSWORD"].Value)
	assert.Equal(t, model.VisibilityMasked, safe["REDIS_PASSWORD"].Visibility)
	assert.Equal(t, "[hidden]", safe["DATABASE_URL"].Value)
	assert.Equal(t, model.VisibilityHidden, safe["DATABASE_URL"].Visibility)
	assert.Equal(t, "[masked]", safe["CUSTOM_SERVICE_TOKEN"].Value)

	insecure := renderedByKey(RenderDotenv(state, model.RenderPolicy{Insecure: true}))
	assert.Equal(t, "secret-redis", insecure["REDIS_PASSWORD"].Value)
	assert.Equal(t, "postgres://example", insecure["DATABASE_URL"].Value)
	assert.Equal(t, "token-value", insecure["CUSTOM_SERVICE_TOKEN"].Value)
}

func TestIngestDotenv_MaterializesDeclaredMissingField(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	missingPassword := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "default", Field: "password"}
	state := IngestDotenv(map[string]string{
		"REDIS_HOST": "localhost",
	}, DotenvIngestOptions{
		Source:       model.Source{Name: ".env", Kind: "dotenv"},
		Clock:        func() time.Time { return now },
		OperationIDs: model.NewMonotonicOperationIDGenerator("mat-op"),
		Declarations: []FieldDeclaration{
			{
				FieldRef: missingPassword,
				Key:      "REDIS_PASSWORD",
				Required: true,
				Source:   model.Source{Name: ".env.example", Kind: "dotenv-spec"},
			},
		},
	})

	require.Contains(t, state.Values, missingPassword)
	assert.Equal(t, model.VisibilityUnresolved, state.Values[missingPassword].Visibility)
	assert.Equal(t, model.SensitivitySensitive, state.Values[missingPassword].Sensitivity)
	assert.Equal(t, model.OperationID("mat-op-000002"), state.Values[missingPassword].LastOperationID)
	assert.Contains(t, diagnosticCodes(state.Diagnostics), "dotenv.unresolved-required")
	require.Len(t, state.Operations, 2)
	assert.Equal(t, model.OperationKindNormalize, state.Operations[1].Kind)

	rendered := RenderDotenvProjection(state, model.RenderPolicy{Insecure: true})
	assert.NotContains(t, renderedByKey(rendered.Variables), "REDIS_PASSWORD")
	assert.Contains(t, diagnosticCodes(rendered.Diagnostics), "dotenv.render-unresolved")
}

func TestIngestDotenv_CollidingProjectionKeepsFirstValue(t *testing.T) {
	t.Parallel()

	state := IngestDotenv(map[string]string{
		"DEFAULT_REDIS_HOST": "later.local",
		"REDIS_HOST":         "localhost",
	}, DotenvIngestOptions{
		Clock:        func() time.Time { return time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC) },
		OperationIDs: model.NewMonotonicOperationIDGenerator("collision-op"),
	})

	defaultHost := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "default", Field: "host"}
	require.Contains(t, state.Values, defaultHost)
	assert.Equal(t, "later.local", state.Values[defaultHost].Resolved)
	assert.Contains(t, diagnosticCodes(state.Diagnostics), "dotenv.collision")
}

func TestFieldRefString(t *testing.T) {
	t.Parallel()

	ref := model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "queues", Field: "host"}
	assert.Equal(t, `universe/redis("queues").host`, ref.String())
}

func diagnosticCodes(diagnostics []model.Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func renderedByKey(rendered []model.RenderedVariable) map[string]model.RenderedVariable {
	result := make(map[string]model.RenderedVariable, len(rendered))
	for _, item := range rendered {
		result[item.Key] = item
	}
	return result
}
