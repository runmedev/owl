package resolver

import (
	"context"

	"github.com/runmedev/owl/internal/model"
)

type Resolver interface {
	Describe(context.Context) (Descriptor, error)
	Resolve(context.Context, Request) (Result, error)
}

type Descriptor struct {
	ID           model.ResolverID
	Name         string
	Description  string
	Capabilities []Capability
}

type Capability string

const (
	CapabilityValueLookup Capability = "value_lookup"
	CapabilityInteraction Capability = "interaction"
)

type Request struct {
	Need       model.UnresolvedNeed
	Policy     Policy
	State      model.EffectiveState
	SourceHint model.Source
}

type Policy struct {
	AllowInteraction bool
	AllowNetwork     bool
}

type Result struct {
	Outcome     model.ResolverAttemptOutcome
	Proposed    *ProposedValue
	Diagnostics []model.Diagnostic
	NextAction  *NextAction
}

type ProposedValue struct {
	Value       string
	Source      model.Source
	Sensitivity model.Sensitivity
	Exposure    model.Exposure
}

type NextActionType string

const (
	NextActionPrompt NextActionType = "prompt"
)

type NextAction struct {
	Type   NextActionType
	Prompt *PromptAction
}

type PromptAction struct {
	NeedID          model.UnresolvedNeedID
	FieldRef        model.FieldRef
	ProjectionKey   model.ProjectionKey
	Label           string
	Description     string
	Sensitivity     model.Sensitivity
	Exposure        model.Exposure
	Required        bool
	Blocking        bool
	AllowEmpty      bool
	ValidationHints []ValidationHint
}

type ValidationHint struct {
	TypeID      model.TypeID
	FieldKind   model.FieldKind
	Description string
}

type PromptAnswer struct {
	NeedID model.UnresolvedNeedID
	Value  string
}
