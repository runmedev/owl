package resolver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/owl/internal/model"
)

type scriptedResolver struct {
	descriptor Descriptor
	resolve    func(context.Context, Request) (Result, error)
}

func (r scriptedResolver) Describe(context.Context) (Descriptor, error) {
	return r.descriptor, nil
}

func (r scriptedResolver) Resolve(ctx context.Context, req Request) (Result, error) {
	return r.resolve(ctx, req)
}

func TestRunnerExecutesOrderedResolversAndStopsPerNeed(t *testing.T) {
	t.Parallel()

	var calls []string
	process := scriptedResolver{
		descriptor: Descriptor{ID: "core/process", Capabilities: []Capability{CapabilityValueLookup}},
		resolve: func(_ context.Context, req Request) (Result, error) {
			calls = append(calls, "process:"+string(req.Need.ProjectionKey))
			return Result{Outcome: model.ResolverAttemptNotFound}, nil
		},
	}
	dotenv := scriptedResolver{
		descriptor: Descriptor{ID: "core/dotenv", Capabilities: []Capability{CapabilityValueLookup}},
		resolve: func(_ context.Context, req Request) (Result, error) {
			calls = append(calls, "dotenv:"+string(req.Need.ProjectionKey))
			return Result{
				Outcome: model.ResolverAttemptResolved,
				Proposed: &ProposedValue{
					Value:       "value-" + string(req.Need.ProjectionKey),
					Source:      model.Source{Name: ".env", Kind: "dotenv"},
					Sensitivity: req.Need.Sensitivity,
					Exposure:    req.Need.Exposure,
				},
			}, nil
		},
	}
	prompt := scriptedResolver{
		descriptor: Descriptor{ID: "core/interactive", Capabilities: []Capability{CapabilityInteraction}},
		resolve: func(_ context.Context, req Request) (Result, error) {
			calls = append(calls, "prompt:"+string(req.Need.ProjectionKey))
			return Result{Outcome: model.ResolverAttemptNotFound}, nil
		},
	}

	result, err := testRunner(process, dotenv, prompt).Resolve(context.Background(), RunRequest{
		State:  stateWithFrontier(optionalNeed("SENTRY_DSN"), unresolvedNeed()),
		Policy: Policy{AllowInteraction: true},
		Chain: ChainConfig{Resolvers: []ResolverConfig{
			{ID: "core/process", Enabled: true},
			{ID: "core/dotenv", Enabled: true},
			{ID: "core/interactive", Enabled: true},
		}},
		SourceHint: model.Source{Name: "[test]", Kind: "test"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"process:API_KEY",
		"dotenv:API_KEY",
		"process:SENTRY_DSN",
		"dotenv:SENTRY_DSN",
	}, calls)
	require.Len(t, result.Proposals, 2)
	assert.Equal(t, model.ResolverAttemptID("attempt-000002"), result.Proposals[0].AttemptID)
	assert.Equal(t, model.ResolverAttemptID("attempt-000004"), result.Proposals[1].AttemptID)
	assert.Len(t, result.Attempts, 4)
	assert.Empty(t, result.Actions)
}

func TestRunnerRecordsPolicyDeniedAttemptAndContinues(t *testing.T) {
	t.Parallel()

	prompt := scriptedResolver{
		descriptor: Descriptor{ID: "core/interactive", Capabilities: []Capability{CapabilityInteraction}},
		resolve: func(context.Context, Request) (Result, error) {
			t.Fatal("policy-denied resolver should not execute")
			return Result{}, nil
		},
	}
	dotenv := resolvingResolver("core/dotenv")

	result, err := testRunner(prompt, dotenv).Resolve(context.Background(), RunRequest{
		State:  stateWithFrontier(unresolvedNeed()),
		Policy: Policy{AllowInteraction: false},
	})
	require.NoError(t, err)

	require.Len(t, result.Attempts, 2)
	assert.Equal(t, model.ResolverAttemptDeniedByOwlPolicy, result.Attempts[0].Outcome)
	assert.Equal(t, model.ResolverID("core/interactive"), result.Attempts[0].ResolverID)
	assert.Equal(t, model.ResolverAttemptResolved, result.Attempts[1].Outcome)
	require.Len(t, result.Proposals, 1)
}

func TestRunnerStopsNeedOnNextAction(t *testing.T) {
	t.Parallel()

	prompt := scriptedResolver{
		descriptor: Descriptor{ID: "core/interactive", Capabilities: []Capability{CapabilityInteraction}},
		resolve: func(_ context.Context, req Request) (Result, error) {
			return Result{
				Outcome: model.ResolverAttemptActionRequired,
				NextAction: &NextAction{
					Type:   NextActionPrompt,
					Prompt: &PromptAction{NeedID: req.Need.ID},
				},
			}, nil
		},
	}
	later := scriptedResolver{
		descriptor: Descriptor{ID: "core/later", Capabilities: []Capability{CapabilityValueLookup}},
		resolve: func(context.Context, Request) (Result, error) {
			t.Fatal("next action should stop this need")
			return Result{}, nil
		},
	}

	result, err := testRunner(prompt, later).Resolve(context.Background(), RunRequest{
		State:  stateWithFrontier(unresolvedNeed()),
		Policy: Policy{AllowInteraction: true},
	})
	require.NoError(t, err)

	require.Len(t, result.Actions, 1)
	assert.Equal(t, NextActionPrompt, result.Actions[0].Type)
	assert.Len(t, result.Attempts, 1)
	assert.Empty(t, result.Proposals)
}

func TestRunnerRecordsProviderErrorAndContinues(t *testing.T) {
	t.Parallel()

	failing := scriptedResolver{
		descriptor: Descriptor{ID: "core/failing", Capabilities: []Capability{CapabilityValueLookup}},
		resolve: func(context.Context, Request) (Result, error) {
			return Result{}, errors.New("backend is down")
		},
	}
	resolving := resolvingResolver("core/dotenv")

	result, err := testRunner(failing, resolving).Resolve(context.Background(), RunRequest{
		State: stateWithFrontier(unresolvedNeed()),
	})
	require.NoError(t, err)

	require.Len(t, result.Attempts, 2)
	assert.Equal(t, model.ResolverAttemptFailed, result.Attempts[0].Outcome)
	assert.Equal(t, "resolver.failed", result.Attempts[0].Diagnostics[0].Code)
	assert.Equal(t, model.ResolverAttemptResolved, result.Attempts[1].Outcome)
	require.Len(t, result.Proposals, 1)
}

func TestRunnerReturnsContextCancellation(t *testing.T) {
	t.Parallel()

	canceling := scriptedResolver{
		descriptor: Descriptor{ID: "core/cancel", Capabilities: []Capability{CapabilityValueLookup}},
		resolve: func(context.Context, Request) (Result, error) {
			return Result{}, context.Canceled
		},
	}

	_, err := testRunner(canceling).Resolve(context.Background(), RunRequest{
		State: stateWithFrontier(unresolvedNeed()),
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRunnerRejectsUnknownConfiguredResolver(t *testing.T) {
	t.Parallel()

	_, err := testRunner(resolvingResolver("core/dotenv")).Resolve(context.Background(), RunRequest{
		State: stateWithFrontier(unresolvedNeed()),
		Chain: ChainConfig{Resolvers: []ResolverConfig{
			{ID: "core/missing", Enabled: true},
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resolver")
}

func testRunner(resolvers ...Resolver) Runner {
	now := time.Date(2026, 8, 3, 20, 0, 0, 0, time.UTC)
	return Runner{
		Resolvers:    resolvers,
		NewAttemptID: modelAttemptIDGenerator(),
		Clock:        func() time.Time { return now },
	}
}

func modelAttemptIDGenerator() func() model.ResolverAttemptID {
	next := 0
	return func() model.ResolverAttemptID {
		next++
		return model.ResolverAttemptID(fmt.Sprintf("attempt-%06d", next))
	}
}

func resolvingResolver(id model.ResolverID) Resolver {
	return scriptedResolver{
		descriptor: Descriptor{ID: id, Capabilities: []Capability{CapabilityValueLookup}},
		resolve: func(_ context.Context, req Request) (Result, error) {
			return Result{
				Outcome: model.ResolverAttemptResolved,
				Proposed: &ProposedValue{
					Value:       "resolved",
					Source:      model.Source{Name: ".env", Kind: "dotenv"},
					Sensitivity: req.Need.Sensitivity,
					Exposure:    req.Need.Exposure,
				},
			}, nil
		},
	}
}

func stateWithFrontier(needs ...model.UnresolvedNeed) model.EffectiveState {
	state := model.NewEffectiveState()
	state.UnresolvedFrontier = model.UnresolvedFrontier{Needs: needs}
	return state
}

func optionalNeed(key model.ProjectionKey) model.UnresolvedNeed {
	ref := model.FieldRef{TypeID: model.TypeCorePlain, Instance: "default", Field: "sentry.dsn"}
	return model.UnresolvedNeed{
		ID:            model.NewUnresolvedNeedID(ref, key, model.UnresolvedReasonMissing),
		FieldRef:      ref,
		ProjectionKey: key,
		Reason:        model.UnresolvedReasonMissing,
		Sensitivity:   model.SensitivityPlaintext,
		Exposure:      model.ExposureClear,
	}
}
