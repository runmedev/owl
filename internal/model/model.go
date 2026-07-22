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

type Exposure string

const (
	ExposureOpaque Exposure = "opaque"
	ExposureKnown  Exposure = "known"
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
	Exposure             Exposure
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

type Visibility string

const (
	VisibilityLiteral    Visibility = "literal"
	VisibilityUnresolved Visibility = "unresolved"
	VisibilityMasked     Visibility = "masked"
	VisibilityHidden     Visibility = "hidden"
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
	FieldRef        FieldRef
	Original        string
	Resolved        string
	Visibility      Visibility
	Sensitivity     Sensitivity
	Exposure        Exposure
	Origin          Source
	Source          Source
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastOperationID OperationID
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
