package registry

import (
	"fmt"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"

	"github.com/runmedev/owl/internal/model"
)

type cueTypeSpec struct {
	importPath string
	definition string
	name       string
}

var builtInCUETypes = []cueTypeSpec{
	{importPath: "./types/core/opaque", definition: "#Opaque", name: "opaque"},
	{importPath: "./types/core/plain", definition: "#Plain", name: "plain"},
	{importPath: "./types/core/secret", definition: "#Secret", name: "secret"},
	{importPath: "./types/core/url", definition: "#URL", name: "url"},
	{importPath: "./types/universe/redis", definition: "#Redis", name: "redis"},
}

type cueCatalog struct {
	root string
	ctx  *cue.Context

	specs        []cueTypeSpec
	specsByID    map[model.TypeID]cueTypeSpec
	valuesByID   map[model.TypeID]cue.Value
	valuesBySpec map[cueTypeSpec]cue.Value
}

func newCUECatalog(root string) (*cueCatalog, error) {
	if root == "" {
		return nil, fmt.Errorf("cue root is not configured")
	}

	catalog := &cueCatalog{
		root:         root,
		ctx:          cuecontext.New(),
		specs:        append([]cueTypeSpec{}, builtInCUETypes...),
		specsByID:    make(map[model.TypeID]cueTypeSpec, len(builtInCUETypes)),
		valuesByID:   make(map[model.TypeID]cue.Value, len(builtInCUETypes)),
		valuesBySpec: make(map[cueTypeSpec]cue.Value, len(builtInCUETypes)),
	}
	for _, spec := range catalog.specs {
		value, err := catalog.loadValue(spec)
		if err != nil {
			return nil, err
		}
		def := value.LookupPath(cue.ParsePath(spec.definition))
		if err := def.Err(); err != nil {
			return nil, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
		}
		id, err := cueString(def, "id")
		if err != nil {
			return nil, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
		}
		typeID, err := model.ParseTypeID(id)
		if err != nil {
			return nil, fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.definition, err)
		}
		catalog.specsByID[typeID] = spec
		catalog.valuesByID[typeID] = value
		catalog.valuesBySpec[spec] = value
	}
	return catalog, nil
}

func (c *cueCatalog) loadValue(spec cueTypeSpec) (cue.Value, error) {
	cfg := &load.Config{Dir: filepath.Clean(c.root)}
	instances := load.Instances([]string{spec.importPath}, cfg)
	if len(instances) == 0 {
		return cue.Value{}, fmt.Errorf("cue type %s: no instances loaded", spec.importPath)
	}
	if err := instances[0].Err; err != nil {
		return cue.Value{}, fmt.Errorf("cue type %s: %w", spec.importPath, err)
	}

	value := c.ctx.BuildInstance(instances[0])
	if err := value.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("cue type %s: %w", spec.importPath, err)
	}
	return value, nil
}

func (c *cueCatalog) LoadTypeDefs() (map[model.TypeID]model.TypeDef, error) {
	result := make(map[model.TypeID]model.TypeDef, len(c.specs))
	for _, spec := range c.specs {
		value := c.valuesBySpec[spec]
		def, err := cueTypeDefFromValue(spec, value, result)
		if err != nil {
			return nil, err
		}
		result[def.ID] = def
	}
	return result, nil
}

func (c *cueCatalog) ValidateValue(typeID model.TypeID, raw string) error {
	schema, err := c.valueSchema(typeID, "value")
	if err != nil {
		return err
	}
	if !schema.Exists() {
		return nil
	}
	return validateCUECandidates(schema, cueCandidateValues(c.ctx, raw))
}

func (c *cueCatalog) ValidateFieldValue(types map[model.TypeID]model.TypeDef, ref model.FieldRef, raw string) error {
	if ref.Field == "" {
		return nil
	}
	def, ok := types[ref.TypeID]
	if !ok || def.Kind != model.FieldKindObject {
		return nil
	}
	if _, ok := def.Fields[ref.Field]; !ok {
		return nil
	}

	schema, err := c.valueSchema(ref.TypeID, "fields."+ref.Field+".value")
	if err != nil {
		return err
	}
	return validateCUECandidates(schema, cueCandidateValues(c.ctx, raw))
}

func (c *cueCatalog) valueSchema(typeID model.TypeID, path string) (cue.Value, error) {
	spec, ok := c.specsByID[typeID]
	if !ok {
		return cue.Value{}, nil
	}
	value := c.valuesByID[typeID]
	schema := value.LookupPath(cue.ParsePath(spec.definition + "." + path))
	if err := schema.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("cue type %s %s.%s: %w", spec.importPath, spec.definition, path, err)
	}
	return schema, nil
}
