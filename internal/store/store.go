package store

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/projection/dotenv"
)

type Store struct {
	state model.EffectiveState
}

type StoreOption func(*config) error

type config struct {
	envs  []sourceInput
	specs []sourceInput
}

type sourceInput struct {
	name string
	raw  []byte
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

func NewStore(opts ...StoreOption) (*Store, error) {
	cfg := config{}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	envRaw := joinRaw(cfg.envs)
	specRaw := joinRaw(cfg.specs)
	state, err := dotenv.AdaptDotenvFiles(envRaw, specRaw, dotenv.DotenvAdapterOptions{
		EnvSource:  sourceFor(cfg.envs, ".env", "dotenv"),
		SpecSource: sourceFor(cfg.specs, ".env.example", "dotenv-spec"),
	})
	if err != nil {
		return nil, err
	}

	return &Store{state: state}, nil
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

func readSource(name string, r io.Reader) (sourceInput, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return sourceInput{}, fmt.Errorf("read %s: %w", name, err)
	}
	return sourceInput{name: name, raw: raw}, nil
}

func joinRaw(inputs []sourceInput) []byte {
	var b strings.Builder
	for _, input := range inputs {
		if len(input.raw) == 0 {
			continue
		}
		_, _ = b.Write(input.raw)
		if !strings.HasSuffix(string(input.raw), "\n") {
			_ = b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

func sourceFor(inputs []sourceInput, fallback string, kind string) model.Source {
	if len(inputs) == 0 {
		return model.Source{Name: fallback, Kind: kind}
	}
	if len(inputs) == 1 {
		return model.Source{Name: inputs[0].name, Kind: kind}
	}
	return model.Source{Name: inputs[len(inputs)-1].name, Kind: kind}
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
