package store

import (
	"context"
	"errors"
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
	types      registry.TypeProvider
	state      model.EffectiveState
	operations []OperationRecord
}

type Operation interface {
	Apply(context.Context, model.EffectiveState) (model.EffectiveState, error)
}

type RecordedOperation interface {
	Operation
	Record() OperationRecord
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

type DotenvPolicy struct {
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
	Visibility    model.Visibility
	Exposure      model.Exposure
	Description   string
	UpdatedAt     time.Time
	Diagnostics   []model.Diagnostic
}

type CheckResult struct {
	OK          bool
	Diagnostics []model.Diagnostic
}

type GetPolicy struct {
	Reveal bool
}

type GetResult struct {
	Key         string
	Field       model.FieldRef
	Value       string
	Visibility  model.Visibility
	Exposure    model.Exposure
	Source      model.Source
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
	Envelope     *StateEnvelope
}

type LoadOperation struct {
	Input LoadInput
}

func (op LoadOperation) Record() OperationRecord {
	return OperationRecord{Kind: OperationRecordLoad, Load: op.Input}
}

type UpdateOperation struct {
	Source model.Source
	Dotenv []DotenvVariable
}

func (op UpdateOperation) Record() OperationRecord {
	return OperationRecord{Kind: OperationRecordUpdate, Update: op}
}

type DeleteOperation struct {
	Keys   []string
	Source model.Source
}

func (op DeleteOperation) Record() OperationRecord {
	return OperationRecord{Kind: OperationRecordDelete, Delete: op}
}

type OperationRecordKind string

const (
	OperationRecordLoad   OperationRecordKind = "load"
	OperationRecordUpdate OperationRecordKind = "update"
	OperationRecordDelete OperationRecordKind = "delete"
)

type OperationRecord struct {
	Kind   OperationRecordKind
	Load   LoadInput
	Update UpdateOperation
	Delete DeleteOperation
}

type NormalizeOperation struct{}

type ValidateOperation struct {
	Types registry.TypeProvider
}

type StateEnvelope struct {
	ModelVersion string
	State        model.EffectiveState
	Provenance   StateProvenance
}

type StateProvenance struct {
	Sources    []model.Source
	Operations []model.OperationMetadata
}

type DiagnosticError struct {
	Diagnostics []model.Diagnostic
}

func (e DiagnosticError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "owl diagnostics failed"
	}
	return e.Diagnostics[0].Code + ": " + e.Diagnostics[0].Message
}

func Diagnostics(err error) []model.Diagnostic {
	if err == nil {
		return nil
	}
	var diagnosticErr DiagnosticError
	if errors.As(err, &diagnosticErr) {
		return append([]model.Diagnostic{}, diagnosticErr.Diagnostics...)
	}
	return []model.Diagnostic{{
		Severity: model.DiagnosticError,
		Code:     "owl.error",
		Message:  err.Error(),
	}}
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

func WithDotenv(source string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		input, err := readSource(source, r)
		if err != nil {
			return err
		}
		cfg.envs = append(cfg.envs, input)
		return nil
	}
}

func WithEnvSpec(source string, r io.Reader) StoreOption {
	return func(cfg *config) error {
		input, err := readSource(source, r)
		if err != nil {
			return err
		}
		cfg.specs = append(cfg.specs, input)
		return nil
	}
}

func WithTypeProvider(types registry.TypeProvider) StoreOption {
	return func(cfg *config) error {
		cfg.types = types
		return nil
	}
}

func (s *Store) Apply(ctx context.Context, op Operation) (model.EffectiveState, error) {
	if recorded, ok := op.(RecordedOperation); ok {
		s.operations = append(s.operations, recorded.Record())
	}
	state, err := op.Apply(ctx, s.state)
	if err != nil {
		return model.EffectiveState{}, err
	}
	s.state = state
	return state, nil
}

func (op LoadOperation) Apply(context.Context, model.EffectiveState) (model.EffectiveState, error) {
	if op.Input.Envelope != nil {
		return op.Input.Envelope.State, nil
	}
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

func (op UpdateOperation) Apply(_ context.Context, state model.EffectiveState) (model.EffectiveState, error) {
	if state.Values == nil {
		state = model.NewEffectiveState()
	}
	source := op.Source
	if source.Name == "" {
		source = model.Source{Name: ".env", Kind: "dotenv"}
	}
	for _, variable := range op.Dotenv {
		ref, binding, found := findBinding(state.Bindings, variable.Key)
		if !found {
			var diagnostic *model.Diagnostic
			ref, diagnostic = inferDotenvFieldRef(variable.Key)
			if diagnostic != nil {
				state.Diagnostics = append(state.Diagnostics, *diagnostic)
			}
			binding = model.Binding{
				ID:           "update:" + variable.Key,
				FieldRef:     ref,
				ProjectionID: model.ProjectionDotenv,
				Key:          model.ProjectionKey(variable.Key),
				Source:       source,
				Origin:       source,
				Confidence:   model.BindingConfidenceOpaque,
				PreserveKey:  true,
			}
			state.Bindings = append(state.Bindings, binding)
		}
		value := state.Values[ref]
		value.FieldRef = ref
		value.Original = variable.Value
		value.Resolved = variable.Value
		value.Visibility = model.VisibilityLiteral
		if value.Sensitivity == "" {
			value.Sensitivity = inferSensitivityForField(ref)
		}
		if value.Exposure == "" {
			value.Exposure = inferExposureForField(ref)
		}
		if value.Origin.Name == "" {
			value.Origin = binding.Origin
		}
		value.Source = source
		value.UpdatedAt = model.RealClock()
		if value.CreatedAt.IsZero() {
			value.CreatedAt = value.UpdatedAt
		}
		state.Values[ref] = value
	}
	return state, nil
}

func (op DeleteOperation) Apply(_ context.Context, state model.EffectiveState) (model.EffectiveState, error) {
	deleted := make(map[model.FieldRef]struct{}, len(op.Keys))
	for _, key := range op.Keys {
		ref, _, found := findBinding(state.Bindings, key)
		if !found {
			continue
		}
		delete(state.Values, ref)
		deleted[ref] = struct{}{}
	}
	if len(deleted) == 0 {
		return state, nil
	}
	bindings := state.Bindings[:0]
	for _, binding := range state.Bindings {
		if _, ok := deleted[binding.FieldRef]; ok {
			continue
		}
		bindings = append(bindings, binding)
	}
	state.Bindings = bindings
	return state, nil
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
			Visibility:    rendered.visibility,
			Exposure:      value.Exposure,
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
	return s.Dotenv(DotenvPolicy(policy))
}

func (s *Store) Dotenv(policy DotenvPolicy) ([]string, error) {
	rendered := dotenv.RenderDotenvProjection(s.state, model.RenderPolicy{Insecure: policy.Insecure})
	envs := make([]string, 0, len(rendered.Variables))
	for _, variable := range rendered.Variables {
		envs = append(envs, variable.Key+"="+variable.Value)
	}
	sort.Strings(envs)
	return envs, nil
}

func (s *Store) Get(key string, policy GetPolicy) (GetResult, bool, error) {
	ref, binding, found := findBinding(s.state.Bindings, key)
	if !found {
		return GetResult{}, false, nil
	}
	value := s.state.Values[ref]
	rendered := renderSnapshotValue(value, SnapshotPolicy(policy))
	return GetResult{
		Key:         key,
		Field:       ref,
		Value:       rendered.value,
		Visibility:  rendered.visibility,
		Exposure:    value.Exposure,
		Source:      value.Source,
		Diagnostics: diagnosticsFor(s.state.Diagnostics, binding),
	}, true, nil
}

func (s *Store) SensitiveKeys() ([]string, error) {
	var keys []string
	for _, binding := range s.state.Bindings {
		value := s.state.Values[binding.FieldRef]
		if value.Sensitivity == model.SensitivitySensitive {
			keys = append(keys, string(binding.Key))
		}
	}
	sort.Strings(keys)
	return keys, nil
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

func (s *Store) OperationRecords() []OperationRecord {
	return append([]OperationRecord{}, s.operations...)
}

func (s *Store) StateEnvelope() StateEnvelope {
	return StateEnvelope{
		ModelVersion: "owl.store.v2",
		State:        s.state,
		Provenance: StateProvenance{
			Sources:    sourcesFromState(s.state),
			Operations: append([]model.OperationMetadata{}, s.state.Operations...),
		},
	}
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
		for _, declaration := range declarations {
			if declaration.UnknownType == "" {
				continue
			}
			return LoadInput{}, DiagnosticError{Diagnostics: []model.Diagnostic{{
				Severity: model.DiagnosticError,
				Code:     "contract.unknown-type",
				Message:  fmt.Sprintf("unknown env spec type %q", declaration.UnknownType),
				Key:      string(declaration.Key),
				FieldRef: declaration.FieldRef,
			}}}
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

func findBinding(bindings []model.Binding, key string) (model.FieldRef, model.Binding, bool) {
	for _, binding := range bindings {
		if string(binding.Key) == key {
			return binding.FieldRef, binding, true
		}
	}
	return model.FieldRef{}, model.Binding{}, false
}

func sourcesFromState(state model.EffectiveState) []model.Source {
	seen := make(map[model.Source]struct{})
	var sources []model.Source
	add := func(source model.Source) {
		if source.Name == "" && source.Kind == "" {
			return
		}
		if _, ok := seen[source]; ok {
			return
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	for _, binding := range state.Bindings {
		add(binding.Source)
		add(binding.Origin)
	}
	for _, value := range state.Values {
		add(value.Source)
		add(value.Origin)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].Kind != sources[j].Kind {
			return sources[i].Kind < sources[j].Kind
		}
		return sources[i].Name < sources[j].Name
	})
	return sources
}

func inferDotenvFieldRef(key string) (model.FieldRef, *model.Diagnostic) {
	parts := strings.Split(key, "_")
	if len(parts) >= 2 && parts[len(parts)-2] == "REDIS" {
		field, ok := redisField(parts[len(parts)-1])
		if ok {
			instance := "default"
			if len(parts) > 2 {
				instance = strings.ToLower(strings.Join(parts[:len(parts)-2], "_"))
			}
			return model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: instance, Field: field}, nil
		}
	}
	if strings.HasPrefix(key, "REDIS_") {
		field, ok := redisField(strings.TrimPrefix(key, "REDIS_"))
		if ok {
			return model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "default", Field: field}, nil
		}
	}

	ref := model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: opaqueFieldName(key)}
	return ref, &model.Diagnostic{
		Severity: model.DiagnosticInfo,
		Code:     "dotenv.opaque",
		Message:  "dotenv key has no explicit type declaration and remains core/opaque",
		Key:      key,
		FieldRef: ref,
	}
}

func redisField(suffix string) (string, bool) {
	switch suffix {
	case "HOST":
		return "host", true
	case "PORT":
		return "port", true
	case "PASSWORD":
		return "password", true
	default:
		return "", false
	}
}

func opaqueFieldName(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "."))
}

func inferSensitivityForField(ref model.FieldRef) model.Sensitivity {
	if ref.TypeID == model.TypeUniverseRedis && ref.Field == "password" {
		return model.SensitivitySensitive
	}
	if ref.TypeID == model.TypeCoreSecret {
		return model.SensitivitySensitive
	}
	if ref.TypeID == model.TypeCorePlain || ref.TypeID == model.TypeCoreURL || ref.TypeID == model.TypeCoreHost || ref.TypeID == model.TypeCorePort {
		return model.SensitivityNonSensitive
	}
	if ref.TypeID == model.TypeCoreOpaque {
		key := strings.ToUpper(ref.Field)
		switch {
		case strings.Contains(key, "PASSWORD"),
			strings.Contains(key, "SECRET"),
			strings.Contains(key, "TOKEN"),
			strings.Contains(key, "API.KEY"),
			strings.Contains(key, "PRIVATE.KEY"):
			return model.SensitivitySensitive
		default:
			return model.SensitivityUnknown
		}
	}
	return model.SensitivityNonSensitive
}

func inferExposureForField(ref model.FieldRef) model.Exposure {
	if ref.TypeID == model.TypeCoreOpaque {
		return model.ExposureOpaque
	}
	return model.ExposureClear
}

type renderedSnapshotValue struct {
	value      string
	visibility model.Visibility
}

func renderSnapshotValue(value model.Value, policy SnapshotPolicy) renderedSnapshotValue {
	rendered := renderedSnapshotValue{value: value.Resolved, visibility: value.Visibility}
	switch value.Visibility {
	case model.VisibilityUnresolved:
		rendered.value = "[unset]"
	case model.VisibilityMasked:
		rendered.value = "[masked]"
	case model.VisibilityHidden:
		rendered.value = "[hidden]"
	}
	if value.Visibility == model.VisibilityLiteral && !policy.Reveal {
		switch value.Sensitivity {
		case model.SensitivitySensitive:
			rendered.value = "[masked]"
			rendered.visibility = model.VisibilityMasked
		case model.SensitivityUnknown:
			rendered.value = "[hidden]"
			rendered.visibility = model.VisibilityHidden
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
