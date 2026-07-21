package owl

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/runmedev/owl/internal/graph"
	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/store"
)

type (
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

type Store struct {
	runtime *graph.Runtime
	load    store.LoadInput
}

type StoreOption func(*config) error

type config struct {
	envs  []store.SourceBytes
	specs []store.SourceBytes
	types registry.TypeProvider
}

func NewStore(opts ...StoreOption) (*Store, error) {
	cfg := config{types: registry.NewBuiltInRegistry()}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	load, err := store.LoadInputFromSourceBytes(cfg.envs, cfg.specs)
	if err != nil {
		return nil, err
	}
	runtime, err := graph.NewRuntime(cfg.types)
	if err != nil {
		return nil, err
	}
	return &Store{runtime: runtime, load: load}, nil
}

func WithEnvFile(name string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		raw, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		cfg.envs = append(cfg.envs, store.SourceBytes{Name: name, Raw: raw})
		return nil
	}
}

func WithSpecFile(name string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		raw, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		cfg.specs = append(cfg.specs, store.SourceBytes{Name: name, Raw: raw})
		return nil
	}
}

func WithEnvBytes(name string, raw []byte) StoreOption {
	return WithEnvFile(name, bytes.NewReader(raw))
}

func WithSpecBytes(name string, raw []byte) StoreOption {
	return WithSpecFile(name, bytes.NewReader(raw))
}

func WithEnvs(source string, envs ...string) StoreOption {
	raw := strings.Join(envs, "\n")
	if raw != "" {
		raw += "\n"
	}
	return WithEnvFile(source, strings.NewReader(raw))
}

func WithTypeProvider(types registry.TypeProvider) StoreOption {
	return func(cfg *config) error {
		cfg.types = types
		return nil
	}
}

func (s *Store) Snapshot(policy SnapshotPolicy) ([]SnapshotItem, error) {
	return s.runtime.Snapshot(context.Background(), s.load, policy)
}

func (s *Store) Source(policy SourcePolicy) ([]string, error) {
	return s.runtime.Dotenv(context.Background(), s.load, policy)
}

func (s *Store) Check() CheckResult {
	check, err := s.runtime.Check(context.Background(), s.load)
	if err != nil {
		return CheckResult{
			OK: false,
			Diagnostics: []model.Diagnostic{{
				Severity: model.DiagnosticError,
				Code:     "graphql.check-failed",
				Message:  err.Error(),
			}},
		}
	}
	return check
}
