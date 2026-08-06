package builtin

import (
	"context"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/resolver"
)

const ResolverIDPrompt model.ResolverID = "core/interactive"

type PromptResolver struct{}

func NewPromptResolver() PromptResolver {
	return PromptResolver{}
}

func (PromptResolver) Describe(context.Context) (resolver.Descriptor, error) {
	return resolver.Descriptor{
		ID:           ResolverIDPrompt,
		Name:         "Interactive",
		Description:  "asks an interactive client for unresolved values",
		Capabilities: []resolver.Capability{resolver.CapabilityInteraction},
	}, nil
}

func (PromptResolver) Resolve(_ context.Context, req resolver.Request) (resolver.Result, error) {
	if req.Need.ID == "" {
		return resolver.Result{Outcome: model.ResolverAttemptNotApplicable}, nil
	}
	if !req.Policy.AllowInteraction {
		return resolver.Result{Outcome: model.ResolverAttemptDeniedByPolicy}, nil
	}
	return resolver.Result{
		Outcome: model.ResolverAttemptActionRequired,
		NextAction: &resolver.NextAction{
			Type: resolver.NextActionPrompt,
			Prompt: &resolver.PromptAction{
				NeedID:        req.Need.ID,
				FieldRef:      req.Need.FieldRef,
				ProjectionKey: req.Need.ProjectionKey,
				Label:         promptLabel(req.Need),
				Description:   req.Need.Description,
				Sensitivity:   req.Need.Sensitivity,
				Exposure:      req.Need.Exposure,
				Required:      req.Need.Required,
				Blocking:      req.Need.Blocking,
				AllowEmpty:    !req.Need.Required,
				ValidationHints: []resolver.ValidationHint{{
					TypeID:      req.Need.FieldRef.TypeID,
					Description: req.Need.Description,
				}},
			},
		},
	}, nil
}

func promptLabel(need model.UnresolvedNeed) string {
	if need.Description != "" {
		return need.Description
	}
	if need.ProjectionKey != "" {
		return string(need.ProjectionKey)
	}
	return need.FieldRef.String()
}
