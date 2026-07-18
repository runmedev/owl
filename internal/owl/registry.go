package owl

type TypeProvider interface {
	ResolveType(TypeID) (TypeDef, bool)
	ResolveTypeRef(string) (TypeDef, bool, error)
}

type BuiltInRegistry struct {
	types map[TypeID]TypeDef
}

func NewBuiltInRegistry() BuiltInRegistry {
	types := map[TypeID]TypeDef{
		TypeCoreOpaque: {
			ID:          TypeCoreOpaque,
			Version:     "0.1.0",
			Name:        "opaque",
			Kind:        FieldKindScalar,
			Source:      "builtin-go",
			Description: "Unknown string-carried ENV value with unknown semantics and sensitivity.",
		},
		TypeCoreSecret: {
			ID:          TypeCoreSecret,
			Version:     "0.1.0",
			Name:        "secret",
			Kind:        FieldKindScalar,
			Source:      "builtin-go",
			Description: "Sensitive string-carried ENV value.",
		},
		TypeCoreURL: {
			ID:          TypeCoreURL,
			Version:     "0.1.0",
			Name:        "url",
			Kind:        FieldKindScalar,
			Source:      "builtin-go",
			Description: "URL-shaped string-carried ENV value.",
		},
		TypeCoreHost: {
			ID:          TypeCoreHost,
			Version:     "0.1.0",
			Name:        "host",
			Kind:        FieldKindScalar,
			Source:      "builtin-go",
			Description: "Host-shaped string-carried ENV value.",
		},
		TypeCorePort: {
			ID:          TypeCorePort,
			Version:     "0.1.0",
			Name:        "port",
			Kind:        FieldKindScalar,
			Source:      "builtin-go",
			Description: "Port-shaped string-carried ENV value.",
		},
		TypeUniverseRedis: {
			ID:      TypeUniverseRedis,
			Version: "0.1.0",
			Name:    "redis",
			Kind:    FieldKindObject,
			Source:  "builtin-go",
			Fields: map[string]FieldDef{
				"host": {
					Name:                 "host",
					TypeID:               TypeCoreHost,
					Required:             true,
					Sensitivity:          SensitivityNonSensitive,
					SemanticVisibility:   SemanticVisibilityKnown,
					PreferredDotenvKey:   "REDIS_HOST",
					AcceptedDotenvSuffix: []string{"HOST"},
				},
				"port": {
					Name:                 "port",
					TypeID:               TypeCorePort,
					Required:             true,
					Sensitivity:          SensitivityNonSensitive,
					SemanticVisibility:   SemanticVisibilityKnown,
					PreferredDotenvKey:   "REDIS_PORT",
					AcceptedDotenvSuffix: []string{"PORT"},
				},
				"password": {
					Name:                 "password",
					TypeID:               TypeCoreSecret,
					Required:             false,
					Sensitivity:          SensitivitySensitive,
					SemanticVisibility:   SemanticVisibilityKnown,
					PreferredDotenvKey:   "REDIS_PASSWORD",
					AcceptedDotenvSuffix: []string{"PASSWORD"},
				},
			},
		},
	}
	return BuiltInRegistry{types: types}
}

func (r BuiltInRegistry) ResolveType(id TypeID) (TypeDef, bool) {
	def, ok := r.types[id]
	return def, ok
}

func (r BuiltInRegistry) ResolveTypeRef(ref string) (TypeDef, bool, error) {
	id, err := ParseTypeID(ref)
	if err != nil {
		return TypeDef{}, false, err
	}
	def, ok := r.ResolveType(id)
	return def, ok, nil
}
