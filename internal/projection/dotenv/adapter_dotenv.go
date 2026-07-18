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
	values := map[string]string{}
	if len(envRaw) > 0 {
		parsed, _, err := godotenv.UnmarshalBytesWithComments(envRaw)
		if err != nil {
			return EffectiveState{}, err
		}
		values = parsed
	}

	var declarations []FieldDeclaration
	if len(specRaw) > 0 {
		specValues, comments, err := godotenv.UnmarshalBytesWithComments(specRaw)
		if err != nil {
			return EffectiveState{}, err
		}
		declarations = declarationsFromSpecs(ParseRawSpec(specValues, comments), specValues, opts.SpecSource)
	}

	return IngestDotenv(values, DotenvIngestOptions{
		Source:       opts.EnvSource,
		Actor:        opts.Actor,
		Clock:        opts.Clock,
		OperationIDs: opts.OperationIDs,
		Declarations: declarations,
	}), nil
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
			declaration.SemanticVisibility = SemanticVisibilityKnown
		case AtomicNamePlain:
			declaration.FieldRef.TypeID = TypeCorePlain
			declaration.Sensitivity = SensitivityNonSensitive
			declaration.SemanticVisibility = SemanticVisibilityKnown
		case AtomicNameOpaque, "":
			declaration.Sensitivity = SensitivityUnknown
			declaration.SemanticVisibility = SemanticVisibilityOpaque
		default:
			declaration.Sensitivity = SensitivityUnknown
			declaration.SemanticVisibility = SemanticVisibilityKnown
		}

		declarations = append(declarations, declaration)
	}
	return declarations
}
