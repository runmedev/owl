package dotenv

import (
	"sort"

	"github.com/stateful/godotenv"
)

type DotenvAdapterOptions struct {
	EnvSource    Source
	SpecSource   Source
	Actor        string
	Clock        Clock
	OperationIDs OperationIDGenerator
}

func AdaptDotenvFiles(envRaw, specRaw []byte, opts DotenvAdapterOptions) (EffectiveState, error) {
	values, err := ParseDotenvValues(envRaw)
	if err != nil {
		return EffectiveState{}, err
	}

	declarations, err := ParseDotenvSpecDeclarations(specRaw, opts.SpecSource)
	if err != nil {
		return EffectiveState{}, err
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

func ParseDotenvSpecDeclarations(raw []byte, source Source) ([]FieldDeclaration, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	specValues, comments, err := godotenv.UnmarshalBytesWithComments(raw)
	if err != nil {
		return nil, err
	}
	return declarationsFromSpecs(ParseRawSpec(specValues, comments), specValues, source), nil
}

func declarationsFromSpecs(specs Specs, descriptions map[string]string, source Source) []FieldDeclaration {
	if source.Name == "" {
		source = Source{Name: ".env.example", Kind: "dotenv-spec"}
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
			FieldRef:    FieldRef{TypeID: TypeCoreOpaque, Instance: "default", Field: opaqueFieldName(key)},
			Key:         ProjectionKey(key),
			Required:    spec.Required,
			Description: descriptions[key],
			Source:      source,
		}

		switch spec.Name {
		case AtomicNameSecret, AtomicNamePassword:
			declaration.FieldRef.TypeID = TypeCoreSecret
			declaration.Sensitivity = SensitivitySensitive
			declaration.Exposure = ExposureKnown
		case AtomicNamePlain:
			declaration.FieldRef.TypeID = TypeCorePlain
			declaration.Sensitivity = SensitivityNonSensitive
			declaration.Exposure = ExposureKnown
		case AtomicNameOpaque, "":
			declaration.Sensitivity = SensitivityUnknown
			declaration.Exposure = ExposureOpaque
		default:
			declaration.UnknownType = spec.Name
			declaration.Sensitivity = SensitivityUnknown
			declaration.Exposure = ExposureKnown
		}

		declarations = append(declarations, declaration)
	}
	return declarations
}
