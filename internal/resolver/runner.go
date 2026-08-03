package resolver

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/runmedev/owl/internal/model"
)

type Runner struct {
	Resolvers    []Resolver
	NewAttemptID func() model.ResolverAttemptID
	Clock        model.Clock
}

type ChainConfig struct {
	Resolvers []ResolverConfig
}

type ResolverConfig struct {
	ID      model.ResolverID
	Enabled bool
}

type RunRequest struct {
	State      model.EffectiveState
	Policy     Policy
	Chain      ChainConfig
	SourceHint model.Source
}

type RunResult struct {
	Attempts  []model.ResolverAttempt
	Proposals []Proposal
	Actions   []NextAction
}

type Proposal struct {
	NeedID        model.UnresolvedNeedID
	AttemptID     model.ResolverAttemptID
	ResolverID    model.ResolverID
	FieldRef      model.FieldRef
	ProjectionKey model.ProjectionKey
	Value         ProposedValue
}

type configuredResolver struct {
	resolver   Resolver
	descriptor Descriptor
	order      int
}

func (r Runner) Resolve(ctx context.Context, req RunRequest) (RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	configured, err := r.configuredResolvers(ctx, req.Chain)
	if err != nil {
		return RunResult{}, err
	}
	needs := orderedNeeds(req.State.UnresolvedFrontier.Needs)
	newAttemptID := r.NewAttemptID
	if newAttemptID == nil {
		newAttemptID = newMonotonicAttemptIDGenerator()
	}
	clock := r.Clock
	if clock == nil {
		clock = model.RealClock
	}
	var result RunResult
	for _, need := range needs {
		for _, item := range configured {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			startedAt := clock()
			attempt := model.ResolverAttempt{
				ID:            newAttemptID(),
				ResolverID:    item.descriptor.ID,
				FieldRef:      need.FieldRef,
				ProjectionKey: need.ProjectionKey,
				Source:        req.SourceHint,
				StartedAt:     startedAt,
			}
			if !policyAllows(req.Policy, item.descriptor) {
				attempt.Outcome = model.ResolverAttemptDeniedByOwlPolicy
				attempt.FinishedAt = clock()
				result.Attempts = append(result.Attempts, attempt)
				continue
			}
			resolved, err := item.resolver.Resolve(ctx, Request{
				Need:       need,
				Policy:     req.Policy,
				State:      req.State,
				SourceHint: req.SourceHint,
			})
			attempt.FinishedAt = clock()
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return result, err
				}
				attempt.Outcome = model.ResolverAttemptFailed
				attempt.Diagnostics = []model.Diagnostic{{
					Severity: model.DiagnosticError,
					Code:     "resolver.failed",
					Message:  "resolver returned an error",
					Key:      string(need.ProjectionKey),
					FieldRef: need.FieldRef,
					Owner:    model.DiagnosticOwnerValidation,
				}}
				result.Attempts = append(result.Attempts, attempt)
				continue
			}
			attempt.Outcome = resolved.Outcome
			attempt.Diagnostics = append([]model.Diagnostic{}, resolved.Diagnostics...)
			if resolved.Proposed != nil {
				attempt.Source = resolved.Proposed.Source
			}
			result.Attempts = append(result.Attempts, attempt)
			if resolved.Proposed != nil && resolved.Outcome == model.ResolverAttemptResolved {
				result.Proposals = append(result.Proposals, Proposal{
					NeedID:        need.ID,
					AttemptID:     attempt.ID,
					ResolverID:    item.descriptor.ID,
					FieldRef:      need.FieldRef,
					ProjectionKey: need.ProjectionKey,
					Value:         *resolved.Proposed,
				})
				break
			}
			if resolved.NextAction != nil {
				result.Actions = append(result.Actions, *resolved.NextAction)
				break
			}
		}
	}
	return result, nil
}

func (r Runner) configuredResolvers(ctx context.Context, chain ChainConfig) ([]configuredResolver, error) {
	described := make(map[model.ResolverID]configuredResolver, len(r.Resolvers))
	var defaults []configuredResolver
	for index, candidate := range r.Resolvers {
		if candidate == nil {
			return nil, fmt.Errorf("resolver %d is nil", index)
		}
		descriptor, err := candidate.Describe(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe resolver %d: %w", index, err)
		}
		if descriptor.ID == "" {
			return nil, fmt.Errorf("resolver %d has empty id", index)
		}
		item := configuredResolver{resolver: candidate, descriptor: descriptor, order: index}
		described[descriptor.ID] = item
		defaults = append(defaults, item)
	}
	if len(chain.Resolvers) == 0 {
		return defaults, nil
	}
	var configured []configuredResolver
	for index, config := range chain.Resolvers {
		if !config.Enabled {
			continue
		}
		item, ok := described[config.ID]
		if !ok {
			return nil, fmt.Errorf("resolver config %d references unknown resolver %q", index, config.ID)
		}
		item.order = index
		configured = append(configured, item)
	}
	return configured, nil
}

func orderedNeeds(needs []model.UnresolvedNeed) []model.UnresolvedNeed {
	ordered := append([]model.UnresolvedNeed{}, needs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Blocking != ordered[j].Blocking {
			return ordered[i].Blocking
		}
		if ordered[i].ProjectionKey != ordered[j].ProjectionKey {
			return ordered[i].ProjectionKey < ordered[j].ProjectionKey
		}
		return ordered[i].FieldRef.String() < ordered[j].FieldRef.String()
	})
	return ordered
}

func policyAllows(policy Policy, descriptor Descriptor) bool {
	for _, capability := range descriptor.Capabilities {
		if capability == CapabilityInteraction {
			if !policy.AllowInteraction {
				return false
			}
		}
	}
	return true
}

func newMonotonicAttemptIDGenerator() func() model.ResolverAttemptID {
	var next int
	return func() model.ResolverAttemptID {
		next++
		return model.ResolverAttemptID(fmt.Sprintf("attempt-%06d", next))
	}
}
