package registry

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"

	"github.com/runmedev/owl/internal/model"
)

type cueTypeSpec struct {
	importPath string
	definition string
	name       string
	id         model.TypeID
}

var builtInCUETypes = []cueTypeSpec{
	{importPath: "./types/core/opaque", definition: "#Opaque", name: "opaque", id: model.TypeCoreOpaque},
	{importPath: "./types/core/plain", definition: "#Plain", name: "plain", id: model.TypeCorePlain},
	{importPath: "./types/core/secret", definition: "#Secret", name: "secret", id: model.TypeCoreSecret},
	{importPath: "./types/core/url", definition: "#URL", name: "url", id: model.TypeCoreURL},
	{importPath: "./types/universe/redis", definition: "#Redis", name: "redis", id: model.TypeUniverseRedis},
	{importPath: "./types/universe/openai", definition: "#OpenAI", name: "openai", id: model.TypeUniverseOpenAI},
	{importPath: "./types/universe/anthropic", definition: "#Anthropic", name: "anthropic", id: model.TypeUniverseAnthropic},
}

type cueCatalogSource interface {
	LoadConfig() (load.Config, error)
}

type cueCatalog struct {
	loadConfig load.Config
	ctx        *cue.Context

	specs        []cueTypeSpec
	specsByID    map[model.TypeID]cueTypeSpec
	valuesByID   map[model.TypeID]cue.Value
	valuesBySpec map[cueTypeSpec]cue.Value
}

func newEmbeddedCUECatalog() (*cueCatalog, error) {
	return newCUECatalog(embeddedCUECatalogSource{})
}

func newDirectoryCUECatalog(root string) (*cueCatalog, error) {
	return newCUECatalog(directoryCUECatalogSource{root: root})
}

func newCUECatalog(source cueCatalogSource) (*cueCatalog, error) {
	cfg, err := source.LoadConfig()
	if err != nil {
		return nil, err
	}
	return newCUECatalogWithConfig(cfg)
}

func newCUECatalogWithConfig(cfg load.Config) (*cueCatalog, error) {
	catalog := &cueCatalog{
		loadConfig:   cfg,
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
		if typeID != spec.id {
			return nil, fmt.Errorf("cue type %s %s: expected id %q, got %q", spec.importPath, spec.definition, spec.id, typeID)
		}
		if _, exists := catalog.specsByID[typeID]; exists {
			return nil, fmt.Errorf("cue type %s %s: duplicate id %q", spec.importPath, spec.definition, typeID)
		}
		catalog.specsByID[typeID] = spec
		catalog.valuesByID[typeID] = value
		catalog.valuesBySpec[spec] = value
	}
	return catalog, nil
}

func (c *cueCatalog) loadValue(spec cueTypeSpec) (cue.Value, error) {
	cfg := c.loadConfig
	instances := load.Instances([]string{spec.importPath}, &cfg)
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
