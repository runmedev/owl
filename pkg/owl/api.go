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
	DotenvPolicy   = store.DotenvPolicy
	GetPolicy      = store.GetPolicy
	SnapshotItem   = store.SnapshotItem
	GetResult      = store.GetResult
	CheckResult    = store.CheckResult

	TypeID             = model.TypeID
	FieldRef           = model.FieldRef
	Source             = model.Source
	DotenvVariable     = store.DotenvVariable
	EnvContract        = store.EnvContract
	EnvBinding         = store.EnvBinding
	StateEnvelope      = store.StateEnvelope
	StateProvenance    = store.StateProvenance
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
	envs      []store.SourceBytes
	specs     []store.SourceBytes
	contracts []store.EnvContract
	envelope  *store.StateEnvelope
	types     registry.TypeProvider
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
	load.Contracts = append(load.Contracts, cfg.contracts...)
	load.Envelope = cfg.envelope
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

func WithEnvSpecFile(name string, r io.Reader) StoreOption {
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

func WithEnvSpecBytes(name string, raw []byte) StoreOption {
	return WithEnvSpecFile(name, bytes.NewReader(raw))
}

func WithEnvs(source string, envs ...string) StoreOption {
	raw := strings.Join(envs, "\n")
	if raw != "" {
		raw += "\n"
	}
	return WithEnvFile(source, strings.NewReader(raw))
}

func WithEnvContract(contract EnvContract) StoreOption {
	return func(cfg *config) error {
		cfg.contracts = append(cfg.contracts, contract)
		return nil
	}
}

func WithEnvContracts(contracts ...EnvContract) StoreOption {
	return func(cfg *config) error {
		cfg.contracts = append(cfg.contracts, contracts...)
		return nil
	}
}

func WithStateEnvelope(envelope StateEnvelope) StoreOption {
	return func(cfg *config) error {
		cfg.envelope = &envelope
		return nil
	}
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

func (s *Store) Dotenv(policy DotenvPolicy) ([]string, error) {
	return s.runtime.Dotenv(context.Background(), s.load, policy)
}

func (s *Store) Get(key string, policy GetPolicy) (GetResult, bool, error) {
	return s.runtime.Get(context.Background(), s.load, key, policy)
}

func (s *Store) SensitiveKeys() ([]string, error) {
	return s.runtime.SensitiveKeys(context.Background(), s.load)
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

func (s *Store) LoadDotenv(source Source, vars []DotenvVariable) error {
	return s.applyDotenv(source, vars, nil)
}

func (s *Store) LoadDotenvLines(source string, envs ...string) error {
	raw := strings.Join(envs, "\n")
	if raw != "" {
		raw += "\n"
	}
	input, err := store.LoadInputFromSourceBytes([]store.SourceBytes{{Name: source, Raw: []byte(raw)}}, nil)
	if err != nil {
		return err
	}
	return s.LoadDotenv(input.DotenvSource, input.Dotenv)
}

func (s *Store) Update(ctx context.Context, newOrUpdated, deleted []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	raw := strings.Join(newOrUpdated, "\n")
	if raw != "" {
		raw += "\n"
	}
	input, err := store.LoadInputFromSourceBytes([]store.SourceBytes{{Name: "[update]", Raw: []byte(raw)}}, nil)
	if err != nil {
		return err
	}
	return s.applyDotenvWithContext(ctx, input.DotenvSource, input.Dotenv, deleted)
}

func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.applyDotenvWithContext(ctx, Source{}, nil, keys)
}

func (s *Store) StateEnvelope(ctx context.Context) (StateEnvelope, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.runtime.StateEnvelope(ctx, s.load)
}

func (s *Store) GraphQLSchema() (string, error) {
	return s.runtime.SchemaJSON(context.Background())
}

func GraphQLSchema() (string, error) {
	runtime, err := graph.NewRuntime(registry.NewBuiltInRegistry())
	if err != nil {
		return "", err
	}
	return runtime.SchemaJSON(context.Background())
}

func Diagnostics(err error) []Diagnostic {
	return store.Diagnostics(err)
}

func (s *Store) applyDotenv(source Source, vars []DotenvVariable, deleted []string) error {
	return s.applyDotenvWithContext(context.Background(), source, vars, deleted)
}

func (s *Store) applyDotenvWithContext(ctx context.Context, source Source, vars []DotenvVariable, deleted []string) error {
	envelope, err := s.runtime.StateEnvelopeAfter(ctx, s.load, store.LoadInput{
		DotenvSource: source,
		Dotenv:       vars,
	}, deleted)
	if err != nil {
		return err
	}
	s.load = store.LoadInput{Envelope: &envelope}
	return nil
}

func WithSpecFile(name string, r io.Reader) StoreOption {
	return WithEnvSpecFile(name, r)
}

func WithSpecBytes(name string, raw []byte) StoreOption {
	return WithEnvSpecBytes(name, raw)
}
