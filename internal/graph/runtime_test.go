package graph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/store"
)

func TestRuntimeDrivesLoadNormalizeValidateSnapshot(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	items, err := runtime.Snapshot(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv: []store.DotenvVariable{
			{Key: "API_URL", Value: "https://api.example.com"},
			{Key: "API_KEY", Value: "secret"},
			{Key: "REDIS_HOST", Value: "localhost"},
		},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
					{
						Key:         "API_URL",
						FieldRef:    model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "api.url"},
						Projection:  model.ProjectionDotenv,
						Description: "API URL",
						Source:      model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
					},
					{
						Key:        "API_KEY",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
						Projection: model.ProjectionDotenv,
						Required:   true,
						Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
					},
				},
			},
		},
	}, SnapshotPolicy{})
	require.NoError(t, err)

	byName := snapshotByName(items)
	assert.Equal(t, "https://api.example.com", byName["API_URL"].Value)
	assert.Equal(t, model.TypeCorePlain, byName["API_URL"].Type)
	assert.Equal(t, "[masked]", byName["API_KEY"].Value)
	assert.Equal(t, model.TypeCoreSecret, byName["API_KEY"].Type)
	assert.Equal(t, `core/plain("default").api.url`, byName["API_URL"].Field.String())
	assert.Equal(t, model.TypeUniverseRedis, byName["REDIS_HOST"].Type)
}

func TestRuntimeRendersDotenvThroughGraphQL(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	envs, err := runtime.Dotenv(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Dotenv: []store.DotenvVariable{
			{Key: "API_KEY", Value: "secret"},
			{Key: "API_URL", Value: "https://api.example.com"},
		},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: "package.json", Kind: "package-json"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
					{
						Key:        "API_KEY",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
						Projection: model.ProjectionDotenv,
					},
					{
						Key:        "API_URL",
						FieldRef:   model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "api.url"},
						Projection: model.ProjectionDotenv,
					},
				},
			},
		},
	}, DotenvPolicy{Insecure: true})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"API_KEY=secret",
		"API_URL=https://api.example.com",
	}, envs)
}

func TestRuntimeCheckReportsRequiredDiagnostics(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(nil)
	require.NoError(t, err)

	check, err := runtime.Check(context.Background(), store.LoadInput{
		DotenvSource: model.Source{Name: ".env", Kind: "dotenv"},
		Contracts: []store.EnvContract{
			{
				Source:     model.Source{Name: ".env.spec", Kind: "dotenv-spec"},
				Projection: model.ProjectionDotenv,
				Bindings: []store.EnvBinding{
					{
						Key:        "API_KEY",
						FieldRef:   model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"},
						Projection: model.ProjectionDotenv,
						Required:   true,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.False(t, check.OK)
	assert.Contains(t, diagnosticCodes(check.Diagnostics), "dotenv.unresolved-required")
}

func snapshotByName(items []SnapshotItem) map[string]SnapshotItem {
	result := make(map[string]SnapshotItem, len(items))
	for _, item := range items {
		result[item.Name] = item
	}
	return result
}

func diagnosticCodes(diagnostics []model.Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}
