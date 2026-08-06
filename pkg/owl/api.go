package owl

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/runmedev/owl/internal/graph"
	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
	"github.com/runmedev/owl/internal/requirements"
	"github.com/runmedev/owl/internal/resolver"
	"github.com/runmedev/owl/internal/resolver/builtin"
	"github.com/runmedev/owl/internal/store"
)

type (
	SnapshotPolicy = store.SnapshotPolicy
	DotenvPolicy   = store.DotenvPolicy
	TypePolicy     = store.TypePolicy
	GetPolicy      = store.GetPolicy
	SnapshotItem   = store.SnapshotItem
	TypeResult     = store.TypeResult
	TypeProposal   = store.TypeProposal
	GetResult      = store.GetResult
	CheckResult    = store.CheckResult
	ResolvePolicy  = resolver.Policy
	ChainConfig    = resolver.ChainConfig
	ResolverConfig = resolver.ResolverConfig
	ResolveResult  = resolver.RunResult
	ResolverAction = resolver.NextAction
	PromptAction   = resolver.PromptAction
	PromptAnswer   = resolver.PromptAnswer

	TypeID                 = model.TypeID
	FieldRef               = model.FieldRef
	ConfigInput            = model.ConfigInput
	NeedInput              = model.NeedInput
	DotenvProjection       = model.DotenvProjectionInput
	DotenvFieldBinding     = model.DotenvFieldBindingInput
	Source                 = model.Source
	DotenvVariable         = store.DotenvVariable
	EnvContract            = store.EnvContract
	EnvBinding             = store.EnvBinding
	StateEnvelope          = store.StateEnvelope
	StateProvenance        = store.StateProvenance
	Visibility             = model.Visibility
	Exposure               = model.Exposure
	Diagnostic             = model.Diagnostic
	DiagnosticSeverity     = model.DiagnosticSeverity
	OperationMetadata      = model.OperationMetadata
	ResolverID             = model.ResolverID
	ResolverAttemptID      = model.ResolverAttemptID
	ResolverAttempt        = model.ResolverAttempt
	ResolverAttemptOutcome = model.ResolverAttemptOutcome
	UnresolvedNeedID       = model.UnresolvedNeedID
	UnresolvedReason       = model.UnresolvedReason
	UnresolvedFrontier     = model.UnresolvedFrontier
	UnresolvedNeed         = model.UnresolvedNeed
	ResolverProposal       = resolver.Proposal
	ProposedValue          = resolver.ProposedValue
)

const (
	TypeCoreOpaque    = model.TypeCoreOpaque
	TypeCorePlain     = model.TypeCorePlain
	TypeCoreSecret    = model.TypeCoreSecret
	TypeCoreURL       = model.TypeCoreURL
	TypeUniverseRedis = model.TypeUniverseRedis

	VisibilityLiteral    = model.VisibilityLiteral
	VisibilityUnresolved = model.VisibilityUnresolved
	VisibilityMasked     = model.VisibilityMasked
	VisibilityHidden     = model.VisibilityHidden

	ExposureOpaque = model.ExposureOpaque
	ExposureClear  = model.ExposureClear

	DiagnosticInfo    = model.DiagnosticInfo
	DiagnosticWarning = model.DiagnosticWarning
	DiagnosticError   = model.DiagnosticError

	ResolverResolved           = model.ResolverAttemptResolved
	ResolverSkipped            = model.ResolverAttemptSkipped
	ResolverNotApplicable      = model.ResolverAttemptNotApplicable
	ResolverNotFound           = model.ResolverAttemptNotFound
	ResolverDeniedByPolicy     = model.ResolverAttemptDeniedByPolicy
	ResolverDeniedByProvider   = model.ResolverAttemptDeniedByProvider
	ResolverIdentityMissing    = model.ResolverAttemptIdentityMissing
	ResolverActionRequired     = model.ResolverAttemptActionRequired
	ResolverNetworkUnavailable = model.ResolverAttemptNetworkUnavailable
	ResolverInvalidResult      = model.ResolverAttemptInvalidResult
	ResolverFailed             = model.ResolverAttemptFailed

	UnresolvedMissing = model.UnresolvedReasonMissing
	UnresolvedInvalid = model.UnresolvedReasonInvalid
)

type Store struct {
	runtime    *graph.Runtime
	types      registry.TypeProvider
	state      model.EffectiveState
	operations []store.OperationRecord
	clock      model.Clock
}

type StoreOption func(*config) error

type config struct {
	envs      []store.SourceBytes
	specs     []store.SourceBytes
	configs   []configInputSource
	contracts []store.EnvContract
	envelope  *store.StateEnvelope
	types     registry.TypeProvider
	clock     model.Clock
}

type configInputSource struct {
	source model.Source
	input  model.ConfigInput
}

type executionInfoKey struct{}

// ExecutionInfo describes the execution context that produced an env update.
type ExecutionInfo struct {
	KnownID     string
	KnownName   string
	ExecContext string
}

type ResolveInput struct {
	Process []DotenvVariable
	Dotenv  []DotenvVariable
	Policy  ResolvePolicy
	Chain   ChainConfig
}

func ContextWithExecutionInfo(ctx context.Context, info ExecutionInfo) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, executionInfoKey{}, info)
}

func ExecutionInfoFromContext(ctx context.Context) (ExecutionInfo, bool) {
	if ctx == nil {
		return ExecutionInfo{}, false
	}
	info, ok := ctx.Value(executionInfoKey{}).(ExecutionInfo)
	return info, ok
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
	for _, source := range cfg.configs {
		contracts, err := requirements.ContractsFromConfig(source.input, source.source, cfg.types)
		if err != nil {
			return nil, err
		}
		load.Contracts = append(load.Contracts, contracts...)
	}
	load.Contracts = append(load.Contracts, cfg.contracts...)
	load.Envelope = cfg.envelope
	runtime, err := graph.NewRuntime(cfg.types)
	if err != nil {
		return nil, err
	}
	clock := cfg.clock
	if clock == nil {
		clock = model.RealClock
	}
	loadTimestamp := clock()
	s := &Store{
		runtime: runtime,
		types:   cfg.types,
		clock:   clock,
		operations: []store.OperationRecord{
			{Kind: store.OperationRecordLoad, Timestamp: loadTimestamp, Load: load},
		},
	}
	if err := s.materialize(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func WithDotenv(source string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		raw, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		cfg.envs = append(cfg.envs, store.SourceBytes{Name: source, Raw: raw})
		return nil
	}
}

func WithEnvSpec(source string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		raw, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		cfg.specs = append(cfg.specs, store.SourceBytes{Name: source, Raw: raw})
		return nil
	}
}

func WithConfig(input ConfigInput) StoreOption {
	return func(cfg *config) error {
		source := model.Source{Name: "[config]", Kind: "owl-config"}
		cfg.configs = append(cfg.configs, configInputSource{
			source: source,
			input:  input,
		})
		return nil
	}
}

func WithConfigSource(source string, input ConfigInput) StoreOption {
	return func(cfg *config) error {
		cfg.configs = append(cfg.configs, configInputSource{
			source: model.Source{Name: source, Kind: "owl-config"},
			input:  input,
		})
		return nil
	}
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

func withClock(clock model.Clock) StoreOption {
	return func(cfg *config) error {
		cfg.clock = clock
		return nil
	}
}

func (s *Store) Snapshot(policy SnapshotPolicy) ([]SnapshotItem, error) {
	return store.NewState(s.state, s.types).Snapshot(policy)
}

func (s *Store) Dotenv(policy DotenvPolicy) ([]string, error) {
	return store.NewState(s.state, s.types).Dotenv(policy)
}

func (s *Store) DotenvSpec() (string, error) {
	if len(s.operations) == 0 || s.operations[0].Kind != store.OperationRecordLoad {
		return requirements.RenderDotenvSpec(nil, s.types)
	}
	return requirements.RenderDotenvSpec(s.operations[0].Load.Contracts, s.types)
}

func (s *Store) Type(policy TypePolicy) (TypeResult, error) {
	return store.NewState(s.state, s.types).Type(policy)
}

func (s *Store) Get(key string, policy GetPolicy) (GetResult, bool, error) {
	return store.NewState(s.state, s.types).Get(key, policy)
}

func (s *Store) SensitiveKeys() ([]string, error) {
	return store.NewState(s.state, s.types).SensitiveKeys()
}

func (s *Store) Check() CheckResult {
	return store.NewState(s.state, s.types).Check()
}

func (s *Store) ResolverAttempts() []ResolverAttempt {
	return append([]ResolverAttempt{}, s.state.ResolverAttempts...)
}

func (s *Store) UnresolvedFrontier() UnresolvedFrontier {
	return UnresolvedFrontier{Needs: append([]UnresolvedNeed{}, s.state.UnresolvedFrontier.Needs...)}
}

func (s *Store) Resolve(ctx context.Context, input ResolveInput) (ResolveResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runner := resolver.Runner{
		Resolvers: []resolver.Resolver{
			builtin.DotenvResolver(catalogsFromVariables(input.Dotenv, Source{Name: ".env", Kind: "dotenv"})...),
			builtin.ProcessResolver(catalogsFromVariables(input.Process, Source{Name: "[process]", Kind: "process"})...),
			builtin.NewPromptResolver(),
		},
		NewAttemptID: publicAttemptIDGenerator(len(s.operations)),
		Clock:        s.clock,
	}
	result, err := runner.Resolve(ctx, resolver.RunRequest{
		State:  s.state,
		Policy: input.Policy,
		Chain:  input.Chain,
	})
	if err != nil {
		return result, err
	}
	if err := s.recordResolverResult(ctx, result); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) ApplyPromptAnswers(ctx context.Context, answers []PromptAnswer) (ResolveResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timestamp := s.clock()
	needs := needsByID(s.state.UnresolvedFrontier.Needs)
	newAttemptID := publicAttemptIDGenerator(len(s.operations))
	var result ResolveResult
	for _, answer := range answers {
		need, ok := needs[answer.NeedID]
		attempt := ResolverAttempt{
			ID:         newAttemptID(),
			ResolverID: builtin.ResolverIDPrompt,
			Outcome:    ResolverResolved,
			Source:     Source{Name: "[interactive]", Kind: "interactive"},
			StartedAt:  timestamp,
			FinishedAt: timestamp,
		}
		if !ok {
			attempt.Outcome = ResolverInvalidResult
			attempt.Message = "interactive answer references an unknown unresolved need"
			result.Attempts = append(result.Attempts, attempt)
			s.operations = append(s.operations, store.OperationRecord{
				Kind:            store.OperationRecordResolverAttempt,
				Timestamp:       timestamp,
				ResolverAttempt: attempt,
			})
			continue
		}
		attempt.FieldRef = need.FieldRef
		attempt.ProjectionKey = need.ProjectionKey
		proposal := ResolverProposal{
			NeedID:        need.ID,
			AttemptID:     attempt.ID,
			ResolverID:    builtin.ResolverIDPrompt,
			FieldRef:      need.FieldRef,
			ProjectionKey: need.ProjectionKey,
			Value: ProposedValue{
				Value:       answer.Value,
				Source:      attempt.Source,
				Sensitivity: need.Sensitivity,
				Exposure:    need.Exposure,
			},
		}
		result.Attempts = append(result.Attempts, attempt)
		result.Proposals = append(result.Proposals, proposal)
		s.operations = append(s.operations,
			store.OperationRecord{
				Kind:            store.OperationRecordResolverAttempt,
				Timestamp:       timestamp,
				ResolverAttempt: attempt,
			},
			store.OperationRecord{
				Kind:             store.OperationRecordApplyResolverProposal,
				Timestamp:        timestamp,
				ResolverProposal: proposal,
			},
		)
	}
	if err := s.materialize(ctx); err != nil {
		return result, err
	}
	return result, nil
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
	return s.applyDotenvWithContext(ctx, sourceFromContext(ctx, input.DotenvSource), input.Dotenv, deleted)
}

func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.applyDotenvWithContext(ctx, sourceFromContext(ctx, Source{}), nil, keys)
}

func (s *Store) StateEnvelope(ctx context.Context) (StateEnvelope, error) {
	return store.NewState(s.state, s.types).StateEnvelope(), nil
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
	if ctx == nil {
		ctx = context.Background()
	}
	if len(vars) == 0 && len(deleted) == 0 {
		return nil
	}
	if len(vars) > 0 {
		timestamp := s.clock()
		s.operations = append(s.operations, store.OperationRecord{
			Kind:      store.OperationRecordUpdate,
			Timestamp: timestamp,
			Update: store.UpdateOperation{
				Source:    source,
				Dotenv:    vars,
				Timestamp: timestamp,
			},
		})
	}
	if len(deleted) > 0 {
		timestamp := s.clock()
		s.operations = append(s.operations, store.OperationRecord{
			Kind:      store.OperationRecordDelete,
			Timestamp: timestamp,
			Delete: store.DeleteOperation{
				Keys:      append([]string{}, deleted...),
				Source:    source,
				Timestamp: timestamp,
			},
		})
	}
	return s.materialize(ctx)
}

func (s *Store) materialize(ctx context.Context) error {
	envelope, err := s.runtime.StateEnvelopeForOperations(ctx, s.operations)
	if err != nil {
		return err
	}
	s.state = envelope.State
	if len(s.state.Operations) == 0 {
		s.state.Operations = append([]model.OperationMetadata{}, envelope.Provenance.Operations...)
	}
	return nil
}

func sourceFromContext(ctx context.Context, fallback Source) Source {
	info, ok := ExecutionInfoFromContext(ctx)
	if !ok {
		return fallback
	}

	name := "[execution]"
	if info.KnownID != "" {
		name = "#" + info.KnownID
	}
	if info.KnownName != "" {
		name = "#" + info.KnownName
	}
	if info.ExecContext != "" {
		name = "[" + info.ExecContext + "]"
	}
	return Source{Name: name, Kind: "execution"}
}

func (s *Store) recordResolverResult(ctx context.Context, result ResolveResult) error {
	timestamp := s.clock()
	for _, attempt := range result.Attempts {
		s.operations = append(s.operations, store.OperationRecord{
			Kind:            store.OperationRecordResolverAttempt,
			Timestamp:       firstTime(attempt.FinishedAt, timestamp),
			ResolverAttempt: attempt,
		})
	}
	for _, proposal := range result.Proposals {
		s.operations = append(s.operations, store.OperationRecord{
			Kind:             store.OperationRecordApplyResolverProposal,
			Timestamp:        timestamp,
			ResolverProposal: proposal,
		})
	}
	if len(result.Attempts) == 0 && len(result.Proposals) == 0 {
		return nil
	}
	return s.materialize(ctx)
}

func catalogsFromVariables(variables []DotenvVariable, fallback Source) []builtin.Catalog {
	positions := make(map[Source]int)
	var catalogs []builtin.Catalog
	for _, variable := range variables {
		source := variable.Source
		if source.Name == "" && source.Kind == "" {
			source = fallback
		}
		index, ok := positions[source]
		if !ok {
			index = len(catalogs)
			positions[source] = index
			catalogs = append(catalogs, builtin.Catalog{Source: source, Values: make(map[model.ProjectionKey]string)})
		}
		catalogs[index].Values[model.ProjectionKey(variable.Key)] = variable.Value
	}
	return catalogs
}

func needsByID(needs []UnresolvedNeed) map[UnresolvedNeedID]UnresolvedNeed {
	result := make(map[UnresolvedNeedID]UnresolvedNeed, len(needs))
	for _, need := range needs {
		result[need.ID] = need
	}
	return result
}

func publicAttemptIDGenerator(offset int) func() ResolverAttemptID {
	var next int
	return func() ResolverAttemptID {
		next++
		return ResolverAttemptID(fmt.Sprintf("attempt-%06d", offset+next))
	}
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}
