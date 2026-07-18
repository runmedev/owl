package dotenv

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	legacy "github.com/runmedev/owl/internal/owl"
)

func TestAdaptDotenvFiles_CompatibleWithOldStoreForSimpleDotenv(t *testing.T) {
	t.Parallel()

	envRaw := []byte(`API_URL=https://api.example.com
MIXPANEL_TOKEN=token-value
DATABASE_URL=postgres://example
`)
	specRaw := []byte(`API_URL="Public API URL" # Plain
MIXPANEL_TOKEN="Mixpanel token" # Secret!
DATABASE_URL="Database URL" # Opaque
`)

	oldStore, err := legacy.NewStore(legacy.WithSpecFile(".env.example", specRaw), legacy.WithEnvFile(".env", envRaw))
	require.NoError(t, err)
	oldValues, err := oldStore.InsecureValues()
	require.NoError(t, err)
	sort.Strings(oldValues)

	state, err := AdaptDotenvFiles(envRaw, specRaw, DotenvAdapterOptions{
		EnvSource:    Source{Name: ".env", Kind: "dotenv"},
		SpecSource:   Source{Name: ".env.example", Kind: "dotenv-spec"},
		Clock:        func() time.Time { return time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC) },
		OperationIDs: NewMonotonicOperationIDGenerator("adapter-op"),
	})
	require.NoError(t, err)

	rendered := RenderDotenvProjection(state, RenderPolicy{Insecure: true})
	assert.Empty(t, rendered.Diagnostics)
	assert.Equal(t, oldValues, renderedAssignments(rendered.Variables))

	safe := renderedByKey(RenderDotenv(state, RenderPolicy{}))
	assert.Equal(t, "https://api.example.com", safe["API_URL"].Value)
	assert.Equal(t, "[masked]", safe["MIXPANEL_TOKEN"].Value)
	assert.Equal(t, "[hidden]", safe["DATABASE_URL"].Value)
}

func TestAdaptDotenvFiles_MaterializesMissingSpecAsUnresolved(t *testing.T) {
	t.Parallel()

	state, err := AdaptDotenvFiles(
		[]byte("API_URL=https://api.example.com\n"),
		[]byte("MIXPANEL_TOKEN=\"Mixpanel token\" # Secret!\n"),
		DotenvAdapterOptions{
			Clock:        func() time.Time { return time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC) },
			OperationIDs: NewMonotonicOperationIDGenerator("adapter-op"),
		},
	)
	require.NoError(t, err)

	ref := FieldRef{TypeID: TypeCoreSecret, Instance: "default", Field: "mixpanel.token"}
	require.Contains(t, state.Values, ref)
	assert.Equal(t, ValueStatusUnresolved, state.Values[ref].Status)
	assert.Contains(t, diagnosticCodes(RenderDotenvProjection(state, RenderPolicy{Insecure: true}).Diagnostics), "dotenv.render-unresolved")
}

func renderedAssignments(rendered []RenderedVariable) []string {
	result := make([]string, 0, len(rendered))
	for _, item := range rendered {
		result = append(result, item.Key+"="+item.Value)
	}
	sort.Strings(result)
	return result
}

func TestDeclarationsFromSpecs_UsesStableKeys(t *testing.T) {
	t.Parallel()

	specs := legacy.ParseRawSpec(
		map[string]string{
			"MIXPANEL_TOKEN": "Mixpanel token",
			"API_URL":        "Public API URL",
		},
		map[string]string{
			"MIXPANEL_TOKEN": "Secret!",
			"API_URL":        "Plain",
		},
	)

	declarations := declarationsFromSpecs(specs, map[string]string{
		"MIXPANEL_TOKEN": "Mixpanel token",
		"API_URL":        "Public API URL",
	}, Source{Name: ".env.example", Kind: "dotenv-spec"})
	keys := make([]string, 0, len(declarations))
	for _, declaration := range declarations {
		keys = append(keys, string(declaration.Key)+":"+string(declaration.FieldRef.TypeID))
	}

	assert.True(t, sort.StringsAreSorted(keys))
	assert.Equal(t, []string{
		"API_URL:" + string(TypeCoreOpaque),
		"MIXPANEL_TOKEN:" + string(TypeCoreSecret),
	}, keys)
	assert.False(t, strings.Contains(strings.Join(keys, ","), "Plain"))
}
