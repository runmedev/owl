package registry

import (
	"github.com/runmedev/owl/internal/model"
)

type TypeProvider interface {
	ResolveType(model.TypeID) (model.TypeDef, bool)
	ResolveTypeRef(string) (model.TypeDef, bool, error)
}

type ValueValidator interface {
	ValidateValue(model.TypeID, string) error
}

type FieldValueValidator interface {
	ValidateFieldValue(model.FieldRef, string) error
}

type BuiltInRegistry struct {
	types map[model.TypeID]model.TypeDef
	cue   *cueCatalog
}

func NewBuiltInRegistry() BuiltInRegistry {
	types := map[model.TypeID]model.TypeDef{
		model.TypeCoreOpaque: {
			ID:          model.TypeCoreOpaque,
			Version:     "0.1.0",
			Name:        "opaque",
			Kind:        model.FieldKindScalar,
			Sensitivity: model.SensitivityUnknown,
			Source:      "builtin-go",
			Description: "Unknown string-carried ENV value with unknown semantics and sensitivity.",
		},
		model.TypeCorePlain: {
			ID:          model.TypeCorePlain,
			Version:     "0.1.0",
			Name:        "plain",
			Kind:        model.FieldKindScalar,
			Sensitivity: model.SensitivityPlaintext,
			Source:      "builtin-go",
			Description: "Known plaintext string-carried ENV value with no narrower semantic contract.",
		},
		model.TypeCoreSecret: {
			ID:          model.TypeCoreSecret,
			Version:     "0.1.0",
			Name:        "secret",
			Kind:        model.FieldKindScalar,
			Sensitivity: model.SensitivitySensitive,
			Source:      "builtin-go",
			Description: "Sensitive string-carried ENV value.",
		},
		model.TypeCoreURL: {
			ID:          model.TypeCoreURL,
			Version:     "0.1.0",
			Name:        "url",
			Kind:        model.FieldKindScalar,
			Sensitivity: model.SensitivityPlaintext,
			Source:      "builtin-go",
			Description: "URL-shaped string-carried ENV value.",
		},
		model.TypeUniverseRedis: {
			ID:      model.TypeUniverseRedis,
			Version: "0.1.0",
			Name:    "redis",
			Kind:    model.FieldKindObject,
			Source:  "builtin-go",
			Fields: map[string]model.FieldDef{
				"host": {
					Name:                 "host",
					TypeID:               model.TypeCorePlain,
					Required:             true,
					Sensitivity:          model.SensitivityPlaintext,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "REDIS_HOST",
					AcceptedDotenvSuffix: []string{"HOST"},
					Description:          "Redis server hostname",
				},
				"port": {
					Name:                 "port",
					TypeID:               model.TypeCorePlain,
					Required:             true,
					Sensitivity:          model.SensitivityPlaintext,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "REDIS_PORT",
					AcceptedDotenvSuffix: []string{"PORT"},
					Description:          "Redis server port",
				},
				"password": {
					Name:                 "password",
					TypeID:               model.TypeCoreSecret,
					Required:             true,
					Sensitivity:          model.SensitivitySensitive,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "REDIS_PASSWORD",
					AcceptedDotenvSuffix: []string{"PASSWORD"},
					Description:          "Redis password",
				},
			},
		},
		model.TypeUniverseOpenAI: {
			ID:      model.TypeUniverseOpenAI,
			Version: "0.1.0",
			Name:    "openai",
			Kind:    model.FieldKindObject,
			Source:  "builtin-go",
			Fields: map[string]model.FieldDef{
				"apiKey": {
					Name:                 "apiKey",
					TypeID:               model.TypeCoreSecret,
					Required:             true,
					Sensitivity:          model.SensitivitySensitive,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "OPENAI_API_KEY",
					AcceptedDotenvSuffix: []string{"API_KEY"},
					Description:          "OpenAI API key",
				},
				"baseURL": {
					Name:                 "baseURL",
					TypeID:               model.TypeCoreURL,
					Required:             false,
					Sensitivity:          model.SensitivityPlaintext,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "OPENAI_BASE_URL",
					AcceptedDotenvSuffix: []string{"BASE_URL"},
					Description:          "OpenAI API base URL",
				},
				"organization": {
					Name:                 "organization",
					TypeID:               model.TypeCorePlain,
					Required:             false,
					Sensitivity:          model.SensitivityPlaintext,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "OPENAI_ORG_ID",
					AcceptedDotenvSuffix: []string{"ORG_ID"},
					Description:          "OpenAI organization ID",
				},
				"project": {
					Name:                 "project",
					TypeID:               model.TypeCorePlain,
					Required:             false,
					Sensitivity:          model.SensitivityPlaintext,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "OPENAI_PROJECT_ID",
					AcceptedDotenvSuffix: []string{"PROJECT_ID"},
					Description:          "OpenAI project ID",
				},
			},
		},
		model.TypeUniverseAnthropic: {
			ID:      model.TypeUniverseAnthropic,
			Version: "0.1.0",
			Name:    "anthropic",
			Kind:    model.FieldKindObject,
			Source:  "builtin-go",
			Fields: map[string]model.FieldDef{
				"apiKey": {
					Name:                 "apiKey",
					TypeID:               model.TypeCoreSecret,
					Required:             true,
					Sensitivity:          model.SensitivitySensitive,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "ANTHROPIC_API_KEY",
					AcceptedDotenvSuffix: []string{"API_KEY"},
					Description:          "Anthropic API key",
				},
				"baseURL": {
					Name:                 "baseURL",
					TypeID:               model.TypeCoreURL,
					Required:             false,
					Sensitivity:          model.SensitivityPlaintext,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "ANTHROPIC_BASE_URL",
					AcceptedDotenvSuffix: []string{"BASE_URL"},
					Description:          "Anthropic API base URL",
				},
			},
		},
	}
	catalog, err := newEmbeddedCUECatalog()
	if err != nil {
		panic("load embedded CUE catalog: " + err.Error())
	}
	return BuiltInRegistry{types: types, cue: catalog}
}

func (r BuiltInRegistry) ResolveType(id model.TypeID) (model.TypeDef, bool) {
	def, ok := r.types[id]
	return def, ok
}

func (r BuiltInRegistry) ResolveTypeRef(ref string) (model.TypeDef, bool, error) {
	id, err := model.ParseTypeID(ref)
	if err != nil {
		return model.TypeDef{}, false, err
	}
	def, ok := r.ResolveType(id)
	return def, ok, nil
}

func (r BuiltInRegistry) ValidateValue(typeID model.TypeID, value string) error {
	if r.cue == nil {
		return nil
	}
	return r.cue.ValidateValue(typeID, value)
}

func (r BuiltInRegistry) ValidateFieldValue(ref model.FieldRef, value string) error {
	if r.cue == nil {
		return nil
	}
	return r.cue.ValidateFieldValue(r.types, ref, value)
}
