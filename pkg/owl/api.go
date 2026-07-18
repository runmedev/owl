package owl

import (
	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/store"
)

type (
	Store          = store.Store
	StoreOption    = store.StoreOption
	SnapshotPolicy = store.SnapshotPolicy
	SnapshotItem   = store.SnapshotItem
	SourcePolicy   = store.SourcePolicy
	CheckResult    = store.CheckResult

	TypeID             = model.TypeID
	FieldRef           = model.FieldRef
	Source             = model.Source
	ValueStatus        = model.ValueStatus
	Diagnostic         = model.Diagnostic
	DiagnosticSeverity = model.DiagnosticSeverity
)

const (
	TypeCoreOpaque    = model.TypeCoreOpaque
	TypeCorePlain     = model.TypeCorePlain
	TypeCoreSecret    = model.TypeCoreSecret
	TypeCoreURL       = model.TypeCoreURL
	TypeCoreHost      = model.TypeCoreHost
	TypeCorePort      = model.TypeCorePort
	TypeUniverseRedis = model.TypeUniverseRedis

	ValueStatusLiteral    = model.ValueStatusLiteral
	ValueStatusUnresolved = model.ValueStatusUnresolved
	ValueStatusMasked     = model.ValueStatusMasked
	ValueStatusHidden     = model.ValueStatusHidden

	DiagnosticInfo    = model.DiagnosticInfo
	DiagnosticWarning = model.DiagnosticWarning
	DiagnosticError   = model.DiagnosticError
)

var (
	NewStore      = store.NewStore
	WithEnvFile   = store.WithEnvFile
	WithSpecFile  = store.WithSpecFile
	WithEnvBytes  = store.WithEnvBytes
	WithSpecBytes = store.WithSpecBytes
	WithEnvs      = store.WithEnvs
)
