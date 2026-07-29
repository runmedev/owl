package registry

import (
	"fmt"

	"cuelang.org/go/cue"

	"github.com/runmedev/owl/internal/model"
)

func LoadBuiltInCUETypeDefs(root string) (map[model.TypeID]model.TypeDef, error) {
	catalog, err := newCUECatalog(root)
	if err != nil {
		return nil, err
	}
	return catalog.LoadTypeDefs()
}

func NewBuiltInCUERegistry(root string) (BuiltInRegistry, error) {
	catalog, err := newCUECatalog(root)
	if err != nil {
		return BuiltInRegistry{}, err
	}
	types, err := catalog.LoadTypeDefs()
	if err != nil {
		return BuiltInRegistry{}, err
	}
	return BuiltInRegistry{types: types, cue: catalog}, nil
}

func cueTypeDefFromValue(spec cueTypeSpec, value cue.Value) (model.TypeDef, error) {
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
	fields, err := cueFields(def.LookupPath(cue.ParsePath("fields")))
	if err != nil {
		return model.TypeDef{}, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
	}
	typeDef.Fields = fields
	return typeDef, nil
}

func cueFields(value cue.Value) (map[string]model.FieldDef, error) {
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
		visibility, err := cueString(field, "visibility")
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		required, err := cueBoolDefault(field, "required", true)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		fields[name] = model.FieldDef{
			Name:        name,
			TypeID:      typeID,
			Required:    required,
			Sensitivity: sensitivityForVisibility(model.Visibility(visibility)),
			Exposure:    exposureForVisibility(model.Visibility(visibility)),
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

func cueBoolDefault(value cue.Value, path string, fallback bool) (bool, error) {
	field := value.LookupPath(cue.ParsePath(path))
	if !field.Exists() {
		return fallback, nil
	}
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

func sensitivityForVisibility(visibility model.Visibility) model.Sensitivity {
	switch visibility {
	case model.VisibilityMasked:
		return model.SensitivitySensitive
	case model.VisibilityLiteral:
		return model.SensitivityNonSensitive
	default:
		return model.SensitivityUnknown
	}
}

func exposureForVisibility(visibility model.Visibility) model.Exposure {
	switch visibility {
	case model.VisibilityHidden:
		return model.ExposureOpaque
	default:
		return model.ExposureClear
	}
}
