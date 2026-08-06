package resolver

import (
	"context"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
)

type fakeValueLookupResolver struct {
	value string
	found bool
}

func (r fakeValueLookupResolver) Describe(context.Context) (Descriptor, error) {
	return Descriptor{
		ID:           "core/dotenv",
		Name:         "Dotenv",
		Description:  "looks up values from dotenv sources",
		Capabilities: []Capability{CapabilityValueLookup},
	}, nil
}

func (r fakeValueLookupResolver) Resolve(_ context.Context, req Request) (Result, error) {
	if !r.found {
		return Result{Outcome: model.ResolverAttemptNotFound}, nil
	}
	return Result{
		Outcome: model.ResolverAttemptResolved,
		Proposed: &ProposedValue{
			Value:       r.value,
			Source:      req.SourceHint,
			Sensitivity: req.Need.Sensitivity,
			Exposure:    req.Need.Exposure,
		},
	}, nil
}

type fakePromptResolver struct{}

func (fakePromptResolver) Describe(context.Context) (Descriptor, error) {
	return Descriptor{
		ID:           "core/interactive",
		Name:         "Interactive",
		Description:  "requests a value from an interactive client",
		Capabilities: []Capability{CapabilityInteraction},
	}, nil
}

func (fakePromptResolver) Resolve(_ context.Context, req Request) (Result, error) {
	if !req.Policy.AllowInteraction {
		return Result{Outcome: model.ResolverAttemptDeniedByPolicy}, nil
	}
	return Result{
		Outcome: model.ResolverAttemptActionRequired,
		NextAction: &NextAction{
			Type:   NextActionPrompt,
			Prompt: &PromptAction{NeedID: req.Need.ID},
		},
	}, nil
}

func TestDescriptorShape(t *testing.T) {
	t.Parallel()

	var r Resolver = fakeValueLookupResolver{found: true}
	descriptor, err := r.Describe(context.Background())
	require.NoError(t, err)

	assert.Equal(t, model.ResolverID("core/dotenv"), descriptor.ID)
	assert.Equal(t, "Dotenv", descriptor.Name)
	assert.Equal(t, []Capability{CapabilityValueLookup}, descriptor.Capabilities)
}

func TestValueLookupResolverFitsInterface(t *testing.T) {
	t.Parallel()

	need := unresolvedNeed()
	var r Resolver = fakeValueLookupResolver{value: "secret", found: true}
	result, err := r.Resolve(context.Background(), Request{
		Need:       need,
		Policy:     Policy{},
		State:      model.NewEffectiveState(),
		SourceHint: model.Source{Name: ".env", Kind: "dotenv"},
	})
	require.NoError(t, err)

	require.NotNil(t, result.Proposed)
	assert.Equal(t, model.ResolverAttemptResolved, result.Outcome)
	assert.Equal(t, "secret", result.Proposed.Value)
	assert.Equal(t, need.Sensitivity, result.Proposed.Sensitivity)
	assert.Equal(t, model.Source{Name: ".env", Kind: "dotenv"}, result.Proposed.Source)
}

func TestValueLookupResolverCanReturnNotFound(t *testing.T) {
	t.Parallel()

	var r Resolver = fakeValueLookupResolver{}
	result, err := r.Resolve(context.Background(), Request{Need: unresolvedNeed()})
	require.NoError(t, err)

	assert.Equal(t, model.ResolverAttemptNotFound, result.Outcome)
	assert.Nil(t, result.Proposed)
	assert.Nil(t, result.NextAction)
}

func TestPromptResolverCanBeBlockedByPolicy(t *testing.T) {
	t.Parallel()

	var r Resolver = fakePromptResolver{}
	result, err := r.Resolve(context.Background(), Request{
		Need:   unresolvedNeed(),
		Policy: Policy{AllowInteraction: false},
	})
	require.NoError(t, err)

	assert.Equal(t, model.ResolverAttemptDeniedByPolicy, result.Outcome)
	assert.Nil(t, result.NextAction)
}

func TestPromptResolverCanReturnNextAction(t *testing.T) {
	t.Parallel()

	need := unresolvedNeed()
	var r Resolver = fakePromptResolver{}
	result, err := r.Resolve(context.Background(), Request{
		Need:   need,
		Policy: Policy{AllowInteraction: true},
	})
	require.NoError(t, err)

	require.NotNil(t, result.NextAction)
	require.NotNil(t, result.NextAction.Prompt)
	assert.Equal(t, model.ResolverAttemptActionRequired, result.Outcome)
	assert.Equal(t, NextActionPrompt, result.NextAction.Type)
	assert.Equal(t, need.ID, result.NextAction.Prompt.NeedID)
}

func TestResolverPackageDoesNotImportStoreOrGraph(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imported := range file.Imports {
			unquoted := strings.Trim(imported.Path.Value, `"`)
			assert.NotEqual(t, "github.com/runmedev/owl/internal/store", unquoted)
			assert.NotEqual(t, "github.com/runmedev/owl/internal/graph", unquoted)
		}
	}
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
