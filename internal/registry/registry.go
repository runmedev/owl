package registry

import "github.com/runmedev/owl/internal/model"

type TypeProvider interface {
	ResolveType(model.TypeID) (model.TypeDef, bool)
	ResolveTypeRef(string) (model.TypeDef, bool, error)
}

type BuiltInRegistry struct {
	types map[model.TypeID]model.TypeDef
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
		model.TypeCoreHost: {
			ID:          model.TypeCoreHost,
			Version:     "0.1.0",
			Name:        "host",
			Kind:        model.FieldKindScalar,
			Source:      "builtin-go",
			Description: "Host-shaped string-carried ENV value.",
		},
		model.TypeCorePort: {
			ID:          model.TypeCorePort,
			Version:     "0.1.0",
			Name:        "port",
			Kind:        model.FieldKindScalar,
			Source:      "builtin-go",
			Description: "Port-shaped string-carried ENV value.",
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
					TypeID:               model.TypeCoreHost,
					Required:             true,
					Sensitivity:          model.SensitivityNonSensitive,
					Exposure:             model.ExposureKnown,
					PreferredDotenvKey:   "REDIS_HOST",
					AcceptedDotenvSuffix: []string{"HOST"},
				},
				"port": {
					Name:                 "port",
					TypeID:               model.TypeCorePort,
					Required:             true,
					Sensitivity:          model.SensitivityNonSensitive,
					Exposure:             model.ExposureKnown,
					PreferredDotenvKey:   "REDIS_PORT",
					AcceptedDotenvSuffix: []string{"PORT"},
				},
				"password": {
					Name:                 "password",
					TypeID:               model.TypeCoreSecret,
					Required:             false,
					Sensitivity:          model.SensitivitySensitive,
					Exposure:             model.ExposureKnown,
					PreferredDotenvKey:   "REDIS_PASSWORD",
					AcceptedDotenvSuffix: []string{"PASSWORD"},
				},
			},
		},
	}
	return BuiltInRegistry{types: types}
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
