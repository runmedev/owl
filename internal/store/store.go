package store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/projection/dotenv"
	"github.com/runmedev/owl/internal/registry"
)

type Store struct {
	types registry.TypeProvider
	state model.EffectiveState
}

type Operation interface {
	Apply(context.Context, model.EffectiveState) (model.EffectiveState, error)
}

type StoreOption func(*config) error

type config struct {
	envs  []sourceInput
	specs []sourceInput
	types registry.TypeProvider
}

type sourceInput struct {
	name string
	raw  []byte
}

type SourceBytes struct {
	Name string
	Raw  []byte
}

type SourcePolicy struct {
	Insecure bool
}

type SnapshotPolicy struct {
	Reveal bool
}

type SnapshotItem struct {
	Name          string
	Value         string
	OriginalValue string
	Type          model.TypeID
	Field         model.FieldRef
	Source        model.Source
	Origin        model.Source
	Status        model.ValueStatus
	Description   string
	UpdatedAt     time.Time
	Diagnostics   []model.Diagnostic
}

type CheckResult struct {
	OK          bool
	Diagnostics []model.Diagnostic
}

type DotenvVariable struct {
	Key   string
	Value string
}

type EnvBinding struct {
	FieldRef    model.FieldRef
	Key         string
	Projection  model.ProjectionID
	Required    bool
	Description string
	Source      model.Source
}

type EnvContract struct {
	Source     model.Source
	Projection model.ProjectionID
	Bindings   []EnvBinding
}

type LoadInput struct {
	DotenvSource model.Source
	Dotenv       []DotenvVariable
	Contracts    []EnvContract
}

type LoadOperation struct {
	Input LoadInput
}

type NormalizeOperation struct{}

type ValidateOperation struct {
	Types registry.TypeProvider
}

func NewStore(opts ...StoreOption) (*Store, error) {
	cfg := config{}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if cfg.types == nil {
		cfg.types = registry.NewBuiltInRegistry()
	}

	load, err := LoadInputFromSourceBytes(sourceBytesFromInputs(cfg.envs), sourceBytesFromInputs(cfg.specs))
	if err != nil {
		return nil, err
	}

	store := &Store{types: cfg.types, state: model.NewEffectiveState()}
	if _, err := store.Apply(context.Background(), LoadOperation{Input: load}); err != nil {
		return nil, err
	}
	if _, err := store.Apply(context.Background(), NormalizeOperation{}); err != nil {
		return nil, err
	}
	state, err := store.Apply(context.Background(), ValidateOperation{Types: cfg.types})
	if err != nil {
		return nil, err
	}
	store.state = state
	return store, nil
}

func WithEnvFile(name string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		input, err := readSource(name, r)
		if err != nil {
			return err
		}
		cfg.envs = append(cfg.envs, input)
		return nil
	}
}

func WithSpecFile(name string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		input, err := readSource(name, r)
		if err != nil {
			return err
		}
		cfg.specs = append(cfg.specs, input)
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

func (s *Store) Apply(ctx context.Context, op Operation) (model.EffectiveState, error) {
	state, err := op.Apply(ctx, s.state)
	if err != nil {
		return model.EffectiveState{}, err
	}
	s.state = state
	return state, nil
}

func (op LoadOperation) Apply(context.Context, model.EffectiveState) (model.EffectiveState, error) {
	values := make(map[string]string, len(op.Input.Dotenv))
	for _, variable := range op.Input.Dotenv {
		values[variable.Key] = variable.Value
	}
	declarations := declarationsFromContracts(op.Input.Contracts)
	source := op.Input.DotenvSource
	if source.Name == "" {
		source = model.Source{Name: ".env", Kind: "dotenv"}
	}
	return dotenv.IngestDotenv(values, dotenv.DotenvIngestOptions{
		Source:       source,
		Declarations: declarations,
	}), nil
}

func (NormalizeOperation) Apply(_ context.Context, state model.EffectiveState) (model.EffectiveState, error) {
	// Dotenv ingest currently materializes the normalized effective state. Keep
	// this explicit graph node so the driver can grow richer normalization
	// without changing the public cascade shape.
	return state, nil
}

func (op ValidateOperation) Apply(_ context.Context, state model.EffectiveState) (model.EffectiveState, error) {
	types := op.Types
	if types == nil {
		types = registry.NewBuiltInRegistry()
	}
	state.Diagnostics = append(state.Diagnostics, ValidateState(state, types)...)
	return state, nil
}

func (s *Store) Snapshot(policy SnapshotPolicy) ([]SnapshotItem, error) {
	items := make([]SnapshotItem, 0, len(s.state.Bindings))
	for _, binding := range s.state.Bindings {
		value := s.state.Values[binding.FieldRef]
		rendered := renderSnapshotValue(value, policy)
		items = append(items, SnapshotItem{
			Name:          string(binding.Key),
			Value:         rendered.value,
			OriginalValue: value.Original,
			Type:          value.FieldRef.TypeID,
			Field:         value.FieldRef,
			Source:        value.Source,
			Origin:        value.Origin,
			Status:        rendered.status,
			Description:   binding.Description,
			UpdatedAt:     value.UpdatedAt,
			Diagnostics:   diagnosticsFor(s.state.Diagnostics, binding),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *Store) Source(policy SourcePolicy) ([]string, error) {
	rendered := dotenv.RenderDotenvProjection(s.state, model.RenderPolicy{Insecure: policy.Insecure})
	envs := make([]string, 0, len(rendered.Variables))
	for _, variable := range rendered.Variables {
		envs = append(envs, variable.Key+"="+variable.Value)
	}
	sort.Strings(envs)
	return envs, nil
}

func (s *Store) Check() CheckResult {
	result := CheckResult{
		OK:          true,
		Diagnostics: append([]model.Diagnostic{}, s.state.Diagnostics...),
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == model.DiagnosticError {
			result.OK = false
			break
		}
	}
	return result
}

func (s *Store) State() model.EffectiveState {
	return s.state
}

func NewState(state model.EffectiveState, types registry.TypeProvider) *Store {
	if types == nil {
		types = registry.NewBuiltInRegistry()
	}
	return &Store{types: types, state: state}
}

func readSource(name string, r io.Reader) (sourceInput, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return sourceInput{}, fmt.Errorf("read %s: %w", name, err)
	}
	return sourceInput{name: name, raw: raw}, nil
}

func joinRaw(inputs []SourceBytes) []byte {
	var b strings.Builder
	for _, input := range inputs {
		if len(input.Raw) == 0 {
			continue
		}
		_, _ = b.Write(input.Raw)
		if !strings.HasSuffix(string(input.Raw), "\n") {
			_ = b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

func sourceFor(inputs []SourceBytes, fallback string, kind string) model.Source {
	if len(inputs) == 0 {
		return model.Source{Name: fallback, Kind: kind}
	}
	if len(inputs) == 1 {
		return model.Source{Name: inputs[0].Name, Kind: kind}
	}
	return model.Source{Name: inputs[len(inputs)-1].Name, Kind: kind}
}

func LoadInputFromSourceBytes(envs, specs []SourceBytes) (LoadInput, error) {
	load := LoadInput{
		DotenvSource: sourceFor(envs, ".env", "dotenv"),
	}

	envRaw := joinRaw(envs)
	values, err := dotenv.ParseDotenvValues(envRaw)
	if err != nil {
		return LoadInput{}, err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		load.Dotenv = append(load.Dotenv, DotenvVariable{Key: key, Value: values[key]})
	}

	for _, spec := range specs {
		declarations, err := dotenv.ParseDotenvSpecDeclarations(spec.Raw, model.Source{Name: spec.Name, Kind: "dotenv-spec"})
		if err != nil {
			return LoadInput{}, err
		}
		contract := EnvContract{
			Source:     model.Source{Name: spec.Name, Kind: "dotenv-spec"},
			Projection: model.ProjectionDotenv,
		}
		for _, declaration := range declarations {
			contract.Bindings = append(contract.Bindings, EnvBinding{
				FieldRef:    declaration.FieldRef,
				Key:         string(declaration.Key),
				Projection:  model.ProjectionDotenv,
				Required:    declaration.Required,
				Description: declaration.Description,
				Source:      declaration.Source,
			})
		}
		load.Contracts = append(load.Contracts, contract)
	}

	return load, nil
}

func sourceBytesFromInputs(inputs []sourceInput) []SourceBytes {
	result := make([]SourceBytes, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, SourceBytes{Name: input.name, Raw: input.raw})
	}
	return result
}

func declarationsFromContracts(contracts []EnvContract) []dotenv.FieldDeclaration {
	var declarations []dotenv.FieldDeclaration
	for _, contract := range contracts {
		source := contract.Source
		if source.Name == "" {
			source = model.Source{Name: "env-contract", Kind: "env-contract"}
		}
		for _, binding := range contract.Bindings {
			projection := binding.Projection
			if projection == "" {
				projection = contract.Projection
			}
			if projection == "" {
				projection = model.ProjectionDotenv
			}
			bindingSource := binding.Source
			if bindingSource.Name == "" {
				bindingSource = source
			}
			declarations = append(declarations, dotenv.FieldDeclaration{
				FieldRef:    binding.FieldRef,
				Key:         model.ProjectionKey(binding.Key),
				Required:    binding.Required,
				Description: binding.Description,
				Source:      bindingSource,
			})
		}
	}
	return declarations
}

type renderedSnapshotValue struct {
	value  string
	status model.ValueStatus
}

func renderSnapshotValue(value model.Value, policy SnapshotPolicy) renderedSnapshotValue {
	rendered := renderedSnapshotValue{value: value.Resolved, status: value.Status}
	switch value.Status {
	case model.ValueStatusUnresolved:
		rendered.value = "[unset]"
	case model.ValueStatusMasked:
		rendered.value = "[masked]"
	case model.ValueStatusHidden:
		rendered.value = "[hidden]"
	}
	if value.Status == model.ValueStatusLiteral && !policy.Reveal {
		switch value.Sensitivity {
		case model.SensitivitySensitive:
			rendered.value = "[masked]"
			rendered.status = model.ValueStatusMasked
		case model.SensitivityUnknown:
			rendered.value = "[hidden]"
			rendered.status = model.ValueStatusHidden
		}
	}
	return rendered
}

func diagnosticsFor(diagnostics []model.Diagnostic, binding model.Binding) []model.Diagnostic {
	var result []model.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.FieldRef == binding.FieldRef || diagnostic.Key == string(binding.Key) {
			result = append(result, diagnostic)
		}
	}
	return result
}
