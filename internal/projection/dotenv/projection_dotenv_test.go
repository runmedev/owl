package dotenv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Source:       Source{Name: ".env.local", Kind: "dotenv"},
		Actor:        "test",
		Clock:        func() time.Time { return now },
		OperationIDs: NewMonotonicOperationIDGenerator("test-op"),
	})

	require.Len(t, state.Values, 8)
	require.Len(t, state.Bindings, 8)

	defaultHost := FieldRef{TypeID: TypeUniverseRedis, Instance: "default", Field: "host"}
	require.Contains(t, state.Values, defaultHost)
	assert.Equal(t, "localhost", state.Values[defaultHost].Resolved)
	assert.Equal(t, SensitivityNonSensitive, state.Values[defaultHost].Sensitivity)
	assert.Equal(t, SemanticVisibilityKnown, state.Values[defaultHost].SemanticVisibility)
	assert.Equal(t, OperationID("test-op-000006"), state.Values[defaultHost].LastOperationID)

	queuesPort := FieldRef{TypeID: TypeUniverseRedis, Instance: "queues", Field: "port"}
	require.Contains(t, state.Values, queuesPort)
	assert.Equal(t, "6380", state.Values[queuesPort].Resolved)

	password := FieldRef{TypeID: TypeUniverseRedis, Instance: "default", Field: "password"}
	require.Contains(t, state.Values, password)
	assert.Equal(t, SensitivitySensitive, state.Values[password].Sensitivity)

	databaseURL := FieldRef{TypeID: TypeCoreOpaque, Instance: "default", Field: "database.url"}
	require.Contains(t, state.Values, databaseURL)
	assert.Equal(t, TypeCoreOpaque, state.Values[databaseURL].FieldRef.TypeID)
	assert.Equal(t, SensitivityUnknown, state.Values[databaseURL].Sensitivity)
	assert.Equal(t, SemanticVisibilityOpaque, state.Values[databaseURL].SemanticVisibility)

	token := FieldRef{TypeID: TypeCoreOpaque, Instance: "default", Field: "custom.service.token"}
	require.Contains(t, state.Values, token)
	assert.Equal(t, SensitivitySensitive, state.Values[token].Sensitivity)

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
		Source:       Source{Name: ".env", Kind: "dotenv"},
		Clock:        func() time.Time { return now },
		OperationIDs: NewMonotonicOperationIDGenerator("render-op"),
	})

	safe := renderedByKey(RenderDotenv(state, RenderPolicy{}))
	assert.Equal(t, "localhost", safe["REDIS_HOST"].Value)
	assert.Equal(t, "[masked]", safe["REDIS_PASSWORD"].Value)
	assert.Equal(t, ValueStatusMasked, safe["REDIS_PASSWORD"].Status)
	assert.Equal(t, "[hidden]", safe["DATABASE_URL"].Value)
	assert.Equal(t, ValueStatusHidden, safe["DATABASE_URL"].Status)
	assert.Equal(t, "[masked]", safe["CUSTOM_SERVICE_TOKEN"].Value)

	insecure := renderedByKey(RenderDotenv(state, RenderPolicy{Insecure: true}))
	assert.Equal(t, "secret-redis", insecure["REDIS_PASSWORD"].Value)
	assert.Equal(t, "postgres://example", insecure["DATABASE_URL"].Value)
	assert.Equal(t, "token-value", insecure["CUSTOM_SERVICE_TOKEN"].Value)
}

func TestIngestDotenv_MaterializesDeclaredMissingField(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	missingPassword := FieldRef{TypeID: TypeUniverseRedis, Instance: "default", Field: "password"}
	state := IngestDotenv(map[string]string{
		"REDIS_HOST": "localhost",
	}, DotenvIngestOptions{
		Source:       Source{Name: ".env", Kind: "dotenv"},
		Clock:        func() time.Time { return now },
		OperationIDs: NewMonotonicOperationIDGenerator("mat-op"),
		Declarations: []FieldDeclaration{
			{
				FieldRef: missingPassword,
				Key:      "REDIS_PASSWORD",
				Required: true,
				Source:   Source{Name: ".env.example", Kind: "dotenv-spec"},
			},
		},
	})

	require.Contains(t, state.Values, missingPassword)
	assert.Equal(t, ValueStatusUnresolved, state.Values[missingPassword].Status)
	assert.Equal(t, SensitivitySensitive, state.Values[missingPassword].Sensitivity)
	assert.Equal(t, OperationID("mat-op-000002"), state.Values[missingPassword].LastOperationID)
	assert.Contains(t, diagnosticCodes(state.Diagnostics), "dotenv.unresolved-required")
	require.Len(t, state.Operations, 2)
	assert.Equal(t, OperationKindNormalize, state.Operations[1].Kind)

	rendered := RenderDotenvProjection(state, RenderPolicy{Insecure: true})
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
		OperationIDs: NewMonotonicOperationIDGenerator("collision-op"),
	})

	defaultHost := FieldRef{TypeID: TypeUniverseRedis, Instance: "default", Field: "host"}
	require.Contains(t, state.Values, defaultHost)
	assert.Equal(t, "later.local", state.Values[defaultHost].Resolved)
	assert.Contains(t, diagnosticCodes(state.Diagnostics), "dotenv.collision")
}

func TestFieldRefString(t *testing.T) {
	t.Parallel()

	ref := FieldRef{TypeID: TypeUniverseRedis, Instance: "queues", Field: "host"}
	assert.Equal(t, `universe/redis("queues").host`, ref.String())
}

func diagnosticCodes(diagnostics []Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func renderedByKey(rendered []RenderedVariable) map[string]RenderedVariable {
	result := make(map[string]RenderedVariable, len(rendered))
	for _, item := range rendered {
		result[item.Key] = item
	}
	return result
}
