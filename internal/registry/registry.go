package registry

import (
	"path/filepath"
	"runtime"

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
	types   map[model.TypeID]model.TypeDef
	cueRoot string
}

func NewBuiltInRegistry() BuiltInRegistry {
	types := map[model.TypeID]model.TypeDef{
		model.TypeCoreOpaque: {
			ID:          model.TypeCoreOpaque,
			Version:     "0.1.0",
			Name:        "opaque",
			Kind:        model.FieldKindScalar,
			Source:      "builtin-go",
			Description: "Unknown string-carried ENV value with unknown semantics and sensitivity.",
		},
		model.TypeCorePlain: {
			ID:          model.TypeCorePlain,
			Version:     "0.1.0",
			Name:        "plain",
			Kind:        model.FieldKindScalar,
			Source:      "builtin-go",
			Description: "Known non-sensitive string-carried ENV value with no narrower semantic contract.",
		},
		model.TypeCoreSecret: {
			ID:          model.TypeCoreSecret,
			Version:     "0.1.0",
			Name:        "secret",
			Kind:        model.FieldKindScalar,
			Source:      "builtin-go",
			Description: "Sensitive string-carried ENV value.",
		},
		model.TypeCoreURL: {
			ID:          model.TypeCoreURL,
			Version:     "0.1.0",
			Name:        "url",
			Kind:        model.FieldKindScalar,
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
					Sensitivity:          model.SensitivityNonSensitive,
					Exposure:             model.ExposureClear,
					PreferredDotenvKey:   "REDIS_HOST",
					AcceptedDotenvSuffix: []string{"HOST"},
					Description:          "Redis server hostname",
				},
				"port": {
					Name:                 "port",
					TypeID:               model.TypeCorePlain,
					Required:             true,
					Sensitivity:          model.SensitivityNonSensitive,
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
	}
	return BuiltInRegistry{types: types, cueRoot: sourceRepoRoot()}
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
	return ValidateCUEValue(r.cueRoot, typeID, value)
}

func (r BuiltInRegistry) ValidateFieldValue(ref model.FieldRef, value string) error {
	return ValidateCUEFieldValue(r.cueRoot, r.types, ref, value)
}

func sourceRepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
