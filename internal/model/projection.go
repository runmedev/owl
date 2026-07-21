package model

import "time"

type ProjectionID string

const ProjectionDotenv ProjectionID = "dotenv"

type ProjectionKey string

type ProjectionDirection string

const (
	ProjectionDirectionIngest ProjectionDirection = "ingest"
	ProjectionDirectionRender ProjectionDirection = "render"
	ProjectionDirectionBoth   ProjectionDirection = "both"
)

type ProjectionRule struct {
	ProjectionID ProjectionID
	FieldRef     FieldRef
	Key          ProjectionKey
	Direction    ProjectionDirection
	Preferred    bool
	Aliases      []ProjectionKey
}

type BindingConfidence string

const (
	BindingConfidenceExplicit    BindingConfidence = "explicit"
	BindingConfidenceTypeDerived BindingConfidence = "type-derived"
	BindingConfidenceHeuristic   BindingConfidence = "heuristic"
	BindingConfidenceOpaque      BindingConfidence = "opaque"
)

type Binding struct {
	ID              string
	FieldRef        FieldRef
	ProjectionID    ProjectionID
	Key             ProjectionKey
	Description     string
	Source          Source
	Origin          Source
	Confidence      BindingConfidence
	Explicit        bool
	PreserveKey     bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastOperationID OperationID
}

type RenderedVariable struct {
	Key    string
	Value  string
	Status ValueStatus
}

type RenderedProjection struct {
	Variables   []RenderedVariable
	Diagnostics []Diagnostic
}

type RenderPolicy struct {
	Insecure bool
}
