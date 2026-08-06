package builtin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/resolver"
)

func TestDotenvResolverFindsValueFromFirstMatchingCatalog(t *testing.T) {
	t.Parallel()

	r := DotenvResolver(
		Catalog{
			Source: model.Source{Name: ".env.local", Kind: "dotenv"},
			Values: map[model.ProjectionKey]string{
				"API_KEY": "from-local",
			},
		},
		Catalog{
			Source: model.Source{Name: ".env", Kind: "dotenv"},
			Values: map[model.ProjectionKey]string{
				"API_KEY": "from-env",
			},
		},
	)

	result, err := r.Resolve(context.Background(), resolver.Request{Need: unresolvedNeed()})
	require.NoError(t, err)

	require.NotNil(t, result.Proposed)
	assert.Equal(t, model.ResolverAttemptResolved, result.Outcome)
	assert.Equal(t, "from-local", result.Proposed.Value)
	assert.Equal(t, model.Source{Name: ".env.local", Kind: "dotenv"}, result.Proposed.Source)
}

func TestProcessResolverReturnsNotFound(t *testing.T) {
	t.Parallel()

	r := ProcessResolver(Catalog{
		Source: model.Source{Name: "[process]", Kind: "process"},
		Values: map[model.ProjectionKey]string{
			"OTHER_KEY": "value",
		},
	})

	result, err := r.Resolve(context.Background(), resolver.Request{Need: unresolvedNeed()})
	require.NoError(t, err)

	assert.Equal(t, model.ResolverAttemptNotFound, result.Outcome)
	assert.Nil(t, result.Proposed)
}

func TestValueResolverReturnsNotApplicableWithoutProjectionKey(t *testing.T) {
	t.Parallel()

	r := DotenvResolver()
	result, err := r.Resolve(context.Background(), resolver.Request{
		Need: model.UnresolvedNeed{FieldRef: model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"}},
	})
	require.NoError(t, err)

	assert.Equal(t, model.ResolverAttemptNotApplicable, result.Outcome)
	assert.Nil(t, result.Proposed)
}

func TestBuiltInResolverDescriptors(t *testing.T) {
	t.Parallel()

	processDescriptor, err := ProcessResolver().Describe(context.Background())
	require.NoError(t, err)
	dotenvDescriptor, err := DotenvResolver().Describe(context.Background())
	require.NoError(t, err)

	assert.Equal(t, ResolverIDProcess, processDescriptor.ID)
	assert.Equal(t, ResolverIDDotenv, dotenvDescriptor.ID)
	assert.Equal(t, []resolver.Capability{resolver.CapabilityValueLookup}, processDescriptor.Capabilities)
	assert.Equal(t, []resolver.Capability{resolver.CapabilityValueLookup}, dotenvDescriptor.Capabilities)
}

func unresolvedNeed() model.UnresolvedNeed {
	ref := model.FieldRef{TypeID: model.TypeCoreSecret, Instance: "default", Field: "api.key"}
	return model.UnresolvedNeed{
		ID:            model.NewUnresolvedNeedID(ref, "API_KEY", model.UnresolvedReasonMissing),
		FieldRef:      ref,
		ProjectionKey: "API_KEY",
		Required:      true,
		Blocking:      true,
		Reason:        model.UnresolvedReasonMissing,
		Sensitivity:   model.SensitivitySensitive,
		Exposure:      model.ExposureClear,
	}
}
