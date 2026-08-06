package store

import (
	"sort"

	"github.com/runmedev/owl/internal/model"
)

func BuildUnresolvedFrontier(state model.EffectiveState) model.UnresolvedFrontier {
	return frontierBuilder{
		state:      state,
		attemptIDs: resolverAttemptIDsByTarget(state.ResolverAttempts),
	}.build()
}

type frontierBuilder struct {
	state      model.EffectiveState
	attemptIDs map[frontierTarget][]model.ResolverAttemptID
}

func (b frontierBuilder) build() model.UnresolvedFrontier {
	var needs []model.UnresolvedNeed
	for _, binding := range b.state.Bindings {
		if !binding.Explicit {
			continue
		}
		need, unresolved := b.needFor(binding)
		if !unresolved {
			continue
		}
		needs = append(needs, need)
	}
	sort.SliceStable(needs, func(i, j int) bool {
		if needs[i].Blocking != needs[j].Blocking {
			return needs[i].Blocking
		}
		if needs[i].ProjectionKey != needs[j].ProjectionKey {
			return needs[i].ProjectionKey < needs[j].ProjectionKey
		}
		return needs[i].FieldRef.String() < needs[j].FieldRef.String()
	})
	return model.UnresolvedFrontier{Needs: needs}
}

func (b frontierBuilder) needFor(binding model.Binding) (model.UnresolvedNeed, bool) {
	value, found := b.state.Values[binding.FieldRef]
	reason, unresolved := b.reasonFor(binding, value, found)
	if !unresolved {
		return model.UnresolvedNeed{}, false
	}
	return model.UnresolvedNeed{
		ID:                 model.NewUnresolvedNeedID(binding.FieldRef, binding.Key, reason),
		FieldRef:           binding.FieldRef,
		ProjectionKey:      binding.Key,
		Required:           binding.Required,
		Blocking:           binding.Required,
		Reason:             reason,
		Description:        binding.Description,
		Sensitivity:        b.sensitivityFor(binding, value, found),
		Exposure:           b.exposureFor(binding, value, found),
		Source:             binding.Source,
		Origin:             binding.Origin,
		ResolverAttemptIDs: b.attemptIDsFor(binding),
	}, true
}

func (b frontierBuilder) reasonFor(binding model.Binding, value model.Value, found bool) (model.UnresolvedReason, bool) {
	if !found || value.Visibility == model.VisibilityUnresolved {
		return model.UnresolvedReasonMissing, true
	}
	for _, diagnostic := range b.state.Diagnostics {
		if diagnostic.Severity != model.DiagnosticError {
			continue
		}
		if diagnostic.FieldRef != binding.FieldRef && diagnostic.Key != string(binding.Key) {
			continue
		}
		return model.UnresolvedReasonInvalid, true
	}
	return "", false
}

func (b frontierBuilder) sensitivityFor(binding model.Binding, value model.Value, found bool) model.Sensitivity {
	if found && value.Sensitivity != "" {
		return value.Sensitivity
	}
	return inferSensitivityForField(binding.FieldRef)
}

func (b frontierBuilder) exposureFor(binding model.Binding, value model.Value, found bool) model.Exposure {
	if found && value.Exposure != "" {
		return value.Exposure
	}
	return inferExposureForField(binding.FieldRef)
}

func (b frontierBuilder) attemptIDsFor(binding model.Binding) []model.ResolverAttemptID {
	return append([]model.ResolverAttemptID{}, b.attemptIDs[frontierTarget{
		field: binding.FieldRef,
		key:   binding.Key,
	}]...)
}

type frontierTarget struct {
	field model.FieldRef
	key   model.ProjectionKey
}

func resolverAttemptIDsByTarget(attempts []model.ResolverAttempt) map[frontierTarget][]model.ResolverAttemptID {
	result := make(map[frontierTarget][]model.ResolverAttemptID)
	for _, attempt := range attempts {
		if attempt.ID == "" {
			continue
		}
		target := frontierTarget{field: attempt.FieldRef, key: attempt.ProjectionKey}
		result[target] = append(result[target], attempt.ID)
	}
	return result
}
