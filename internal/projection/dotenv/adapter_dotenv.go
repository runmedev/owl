package dotenv

import (
	"sort"
	"strings"

	"github.com/stateful/godotenv"

	"github.com/runmedev/owl/internal/model"
)

type DotenvAdapterOptions struct {
	EnvSource    model.Source
	SpecSource   model.Source
	Actor        string
	Clock        model.Clock
	OperationIDs model.OperationIDGenerator
}

func AdaptDotenvFiles(envRaw, specRaw []byte, opts DotenvAdapterOptions) (model.EffectiveState, error) {
	values, err := ParseDotenvValues(envRaw)
	if err != nil {
		return model.EffectiveState{}, err
	}

	declarations, err := ParseDotenvSpecDeclarations(specRaw, opts.SpecSource)
	if err != nil {
		return model.EffectiveState{}, err
	}

	return IngestDotenv(values, DotenvIngestOptions{
		Source:       opts.EnvSource,
		Actor:        opts.Actor,
		Clock:        opts.Clock,
		OperationIDs: opts.OperationIDs,
		Declarations: declarations,
	}), nil
}

func ParseDotenvValues(raw []byte) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	parsed, _, err := godotenv.UnmarshalBytesWithComments(raw)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func ParseDotenvSpecDeclarations(raw []byte, source model.Source) ([]FieldDeclaration, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	specValues, comments, err := godotenv.UnmarshalBytesWithComments(raw)
	if err != nil {
		return nil, err
	}
	return declarationsFromSpecs(ParseRawSpec(specValues, comments), specValues, source, orderedDotenvKeys(raw, specValues)), nil
}

func declarationsFromSpecs(specs Specs, descriptions map[string]string, source model.Source, orderedKeys []string) []FieldDeclaration {
	if source.Name == "" {
		source = model.Source{Name: ".env.example", Kind: "dotenv-spec"}
	}

	keys := orderedSpecKeys(specs, orderedKeys)

	declarations := make([]FieldDeclaration, 0, len(keys))
	for index, key := range keys {
		spec := specs[key]
		declaration := FieldDeclaration{
			FieldRef:    model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: opaqueFieldName(key)},
			Key:         model.ProjectionKey(key),
			Required:    spec.Required,
			Description: descriptions[key],
			Source:      source,
			Order:       uint(index + 1),
		}

		switch spec.Name {
		case AtomicNameSecret, AtomicNamePassword:
			declaration.FieldRef.TypeID = model.TypeCoreSecret
			declaration.Sensitivity = model.SensitivitySensitive
			declaration.Exposure = model.ExposureClear
		case AtomicNamePlain:
			declaration.FieldRef.TypeID = model.TypeCorePlain
			declaration.Sensitivity = model.SensitivityNonSensitive
			declaration.Exposure = model.ExposureClear
		case AtomicNameURL, "URL":
			declaration.FieldRef.TypeID = model.TypeCoreURL
			declaration.Sensitivity = model.SensitivityNonSensitive
			declaration.Exposure = model.ExposureClear
		case AtomicNamePort:
			declaration.FieldRef.TypeID = model.TypeCorePort
			declaration.Sensitivity = model.SensitivityNonSensitive
			declaration.Exposure = model.ExposureClear
		case AtomicNameOpaque, "":
			declaration.Sensitivity = model.SensitivityUnknown
			declaration.Exposure = model.ExposureOpaque
		default:
			declaration.UnknownType = spec.Name
			declaration.Sensitivity = model.SensitivityUnknown
			declaration.Exposure = model.ExposureClear
		}

		declarations = append(declarations, declaration)
	}
	return declarations
}

func orderedSpecKeys(specs Specs, orderedKeys []string) []string {
	keys := make([]string, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, key := range orderedKeys {
		if _, ok := specs[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	var remaining []string
	for key := range specs {
		if _, ok := seen[key]; ok {
			continue
		}
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)
	return append(keys, remaining...)
}

func orderedDotenvKeys(raw []byte, parsed map[string]string) []string {
	keys := make([]string, 0, len(parsed))
	seen := make(map[string]struct{}, len(parsed))
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		index := strings.Index(line, "=")
		if index <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:index])
		if _, ok := parsed[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	return keys
}
