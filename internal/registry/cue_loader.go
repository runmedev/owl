package registry

import (
	"fmt"

	"cuelang.org/go/cue"

	"github.com/runmedev/owl/internal/model"
)

func LoadBuiltInCUETypeDefs(root string) (map[model.TypeID]model.TypeDef, error) {
	catalog, err := newDirectoryCUECatalog(root)
	if err != nil {
		return nil, err
	}
	return catalog.LoadTypeDefs()
}

// NewBuiltInRegistryFromDirectory returns the built-in Go registry with value
// validation supplied by a complete directory-backed CUE module.
func NewBuiltInRegistryFromDirectory(root string) (BuiltInRegistry, error) {
	catalog, err := newDirectoryCUECatalog(root)
	if err != nil {
		return BuiltInRegistry{}, err
	}
	return newBuiltInRegistry(catalog), nil
}

func cueTypeDefFromValue(spec cueTypeSpec, value cue.Value, types map[model.TypeID]model.TypeDef) (model.TypeDef, error) {
	def := value.LookupPath(cue.ParsePath(spec.definition))
	if err := def.Err(); err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}

	id, err := cueString(def, "id")
	if err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}
	typeID, err := model.ParseTypeID(id)
	if err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}
	kind, err := cueString(def, "kind")
	if err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}
	description, err := cueString(def, "description")
	if err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}

	typeDef := model.TypeDef{
		ID:          typeID,
		Version:     "0.1.0",
		Name:        spec.name,
		Kind:        cueFieldKind(kind),
		Source:      "builtin-cue",
		Description: description,
	}
	if typeDef.Kind == model.FieldKindScalar {
		sensitivity, err := cueSensitivity(def, "sensitivity")
		if err != nil {
			return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
		}
		typeDef.Sensitivity = sensitivity
	}
	fields, err := cueFields(def.LookupPath(cue.ParsePath("fields")), types)
	if err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}
	typeDef.Fields = fields
	return typeDef, nil
}

func cueFields(value cue.Value, types map[model.TypeID]model.TypeDef) (map[string]model.FieldDef, error) {
	if !value.Exists() {
		return nil, nil
	}
	iter, err := value.Fields()
	if err != nil {
		return nil, err
	}
	fields := make(map[string]model.FieldDef)
	for iter.Next() {
		name := iter.Label()
		field := iter.Value()
		typeRef, err := cueString(field, "type")
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		typeID, err := model.ParseTypeID(typeRef)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		description, err := cueString(field, "description")
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		required, err := cueBool(field, "required")
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		sensitivity, err := cueOptionalSensitivity(field, "sensitivity")
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		if sensitivity == "" {
			sensitivity = inheritedSensitivity(types, typeID)
		}
		fields[name] = model.FieldDef{
			Name:        name,
			TypeID:      typeID,
			Required:    required,
			Sensitivity: sensitivity,
			Exposure:    exposureForSensitivity(sensitivity),
			Description: description,
		}
	}
	return fields, nil
}

func cueString(value cue.Value, path string) (string, error) {
	result, err := value.LookupPath(cue.ParsePath(path)).String()
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return result, nil
}

func cueOptionalString(value cue.Value, path string) (string, bool, error) {
	field := value.LookupPath(cue.ParsePath(path))
	if !field.Exists() {
		return "", false, nil
	}
	result, err := field.String()
	if err != nil {
		return "", true, fmt.Errorf("%s: %w", path, err)
	}
	return result, true, nil
}

func cueBool(value cue.Value, path string) (bool, error) {
	field := value.LookupPath(cue.ParsePath(path))
	defaultValue, ok := field.Default()
	if ok {
		field = defaultValue
	}
	result, err := field.Bool()
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	return result, nil
}

func cueFieldKind(kind string) model.FieldKind {
	if kind == "composite" {
		return model.FieldKindObject
	}
	return model.FieldKindScalar
}

func cueSensitivity(value cue.Value, path string) (model.Sensitivity, error) {
	raw, err := cueString(value, path)
	if err != nil {
		return "", err
	}
	return parseSensitivity(raw)
}

func cueOptionalSensitivity(value cue.Value, path string) (model.Sensitivity, error) {
	raw, ok, err := cueOptionalString(value, path)
	if err != nil || !ok {
		return "", err
	}
	return parseSensitivity(raw)
}

func parseSensitivity(raw string) (model.Sensitivity, error) {
	switch model.Sensitivity(raw) {
	case model.SensitivityUnknown:
		return model.SensitivityUnknown, nil
	case model.SensitivityPlaintext:
		return model.SensitivityPlaintext, nil
	case model.SensitivitySensitive:
		return model.SensitivitySensitive, nil
	default:
		return "", fmt.Errorf("sensitivity: unknown value %q", raw)
	}
}

func inheritedSensitivity(types map[model.TypeID]model.TypeDef, typeID model.TypeID) model.Sensitivity {
	def, ok := types[typeID]
	if !ok || def.Sensitivity == "" {
		return model.SensitivityUnknown
	}
	return def.Sensitivity
}

func exposureForSensitivity(sensitivity model.Sensitivity) model.Exposure {
	if sensitivity == model.SensitivityUnknown {
		return model.ExposureOpaque
	}
	return model.ExposureClear
}
