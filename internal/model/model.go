package model

import (
	"fmt"
	"time"
)

type FieldKind string

const (
	FieldKindScalar FieldKind = "scalar"
	FieldKindObject FieldKind = "object"
)

type Sensitivity string

const (
	SensitivityUnknown      Sensitivity = "unknown"
	SensitivityNonSensitive Sensitivity = "non-sensitive"
	SensitivitySensitive    Sensitivity = "sensitive"
)

type EffectiveVisibility string

const (
	EffectiveVisibilityOpaque EffectiveVisibility = "opaque"
	EffectiveVisibilityKnown  EffectiveVisibility = "known"
)

type TypeDef struct {
	ID          TypeID
	Version     string
	Name        string
	Kind        FieldKind
	Fields      map[string]FieldDef
	Source      string
	Description string
}

type FieldDef struct {
	Name                 string
	TypeID               TypeID
	Required             bool
	Sensitivity          Sensitivity
	EffectiveVisibility  EffectiveVisibility
	PreferredDotenvKey   string
	AcceptedDotenvSuffix []string
	Description          string
}

type TypeInstanceID struct {
	TypeID   TypeID
	Instance string
}

type FieldRef struct {
	TypeID   TypeID
	Instance string
	Field    string
}

func (r FieldRef) String() string {
	alias := r.TypeID.Alias()
	if r.Instance == "" {
		return fmt.Sprintf("%s.%s", alias, r.Field)
	}
	return fmt.Sprintf("%s(%q).%s", alias, r.Instance, r.Field)
}

type ValueStatus string

const (
	ValueStatusLiteral    ValueStatus = "literal"
	ValueStatusUnresolved ValueStatus = "unresolved"
	ValueStatusMasked     ValueStatus = "masked"
	ValueStatusHidden     ValueStatus = "hidden"
)

type Source struct {
	Name string
	Kind string
}

type OperationID string

type OperationKind string

const (
	OperationKindLoad      OperationKind = "load"
	OperationKindSet       OperationKind = "set"
	OperationKindDelete    OperationKind = "delete"
	OperationKindResolve   OperationKind = "resolve"
	OperationKindValidate  OperationKind = "validate"
	OperationKindNormalize OperationKind = "normalize"
	OperationKindRender    OperationKind = "render"
)

type OperationMetadata struct {
	ID           OperationID
	Kind         OperationKind
	Timestamp    time.Time
	Actor        string
	Source       Source
	ProjectionID ProjectionID
}

type Value struct {
	FieldRef            FieldRef
	Original            string
	Resolved            string
	Status              ValueStatus
	Sensitivity         Sensitivity
	EffectiveVisibility EffectiveVisibility
	Origin              Source
	Source              Source
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastOperationID     OperationID
}

type DiagnosticSeverity string

const (
	DiagnosticInfo    DiagnosticSeverity = "info"
	DiagnosticWarning DiagnosticSeverity = "warning"
	DiagnosticError   DiagnosticSeverity = "error"
)

type Diagnostic struct {
	Severity DiagnosticSeverity
	Code     string
	Message  string
	Key      string
	FieldRef FieldRef
}

type EffectiveState struct {
	Values      map[FieldRef]Value
	Bindings    []Binding
	Operations  []OperationMetadata
	Diagnostics []Diagnostic
}

func NewEffectiveState() EffectiveState {
	return EffectiveState{
		Values: make(map[FieldRef]Value),
	}
}

type Clock func() time.Time

func RealClock() time.Time {
	return time.Now().UTC()
}

type OperationIDGenerator func() OperationID

func NewMonotonicOperationIDGenerator(prefix string) OperationIDGenerator {
	var next int
	return func() OperationID {
		next++
		return OperationID(fmt.Sprintf("%s-%06d", prefix, next))
	}
}
