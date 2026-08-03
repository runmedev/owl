package store

import (
	"sort"

	"github.com/runmedev/owl/internal/model"
)

func BuildUnresolvedFrontier(state model.EffectiveState) model.UnresolvedFrontier {
	attemptIDs := resolverAttemptIDsByTarget(state.ResolverAttempts)
	var needs []model.UnresolvedNeed
	for _, binding := range state.Bindings {
		if !binding.Explicit {
			continue
		}
		value, found := state.Values[binding.FieldRef]
		reason, unresolved := unresolvedReason(binding, value, found, state.Diagnostics)
		if !unresolved {
			continue
		}
		needs = append(needs, model.UnresolvedNeed{
			ID:                 model.NewUnresolvedNeedID(binding.FieldRef, binding.Key, reason),
			FieldRef:           binding.FieldRef,
			ProjectionKey:      binding.Key,
			Required:           binding.Required,
			Blocking:           binding.Required,
			Reason:             reason,
			Description:        binding.Description,
			Sensitivity:        sensitivityForNeed(binding, value, found),
			Exposure:           exposureForNeed(binding, value, found),
			Source:             binding.Source,
			Origin:             binding.Origin,
			ResolverAttemptIDs: append([]model.ResolverAttemptID{}, attemptIDs[frontierTarget{field: binding.FieldRef, key: binding.Key}]...),
		})
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

func unresolvedReason(binding model.Binding, value model.Value, found bool, diagnostics []model.Diagnostic) (model.UnresolvedReason, bool) {
	if !found || value.Visibility == model.VisibilityUnresolved {
		return model.UnresolvedReasonMissing, true
	}
	for _, diagnostic := range diagnostics {
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

func sensitivityForNeed(binding model.Binding, value model.Value, found bool) model.Sensitivity {
	if found && value.Sensitivity != "" {
		return value.Sensitivity
	}
	return inferSensitivityForField(binding.FieldRef)
}

func exposureForNeed(binding model.Binding, value model.Value, found bool) model.Exposure {
	if found && value.Exposure != "" {
		return value.Exposure
	}
	return inferExposureForField(binding.FieldRef)
}
