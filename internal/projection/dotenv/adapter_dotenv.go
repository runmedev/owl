package dotenv

import (
	"sort"

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
	return declarationsFromSpecs(ParseRawSpec(specValues, comments), specValues, source), nil
}

func declarationsFromSpecs(specs Specs, descriptions map[string]string, source model.Source) []FieldDeclaration {
	if source.Name == "" {
		source = model.Source{Name: ".env.example", Kind: "dotenv-spec"}
	}

	keys := make([]string, 0, len(specs))
	for key := range specs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	declarations := make([]FieldDeclaration, 0, len(keys))
	for _, key := range keys {
		spec := specs[key]
		declaration := FieldDeclaration{
			FieldRef:    model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: opaqueFieldName(key)},
			Key:         model.ProjectionKey(key),
			Required:    spec.Required,
			Description: descriptions[key],
			Source:      source,
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
