package registry

import (
	"fmt"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"

	"github.com/runmedev/owl/internal/model"
)

func ValidateCUEValue(root string, typeID model.TypeID, raw string) error {
	spec, ok := cueBuiltInTypeForID(typeID)
	if !ok {
		return nil
	}
	schema, ctx, err := loadCUEValueSchema(root, spec, spec.definition+".value")
	if err != nil {
		return err
	}
	return validateCUECandidates(schema, cueCandidateValues(ctx, raw))
}

func ValidateCUEFieldValue(root string, types map[model.TypeID]model.TypeDef, ref model.FieldRef, raw string) error {
	spec, ok := cueBuiltInTypeForID(ref.TypeID)
	if !ok || ref.Field == "" {
		return nil
	}
	def, ok := types[ref.TypeID]
	if !ok || def.Kind != model.FieldKindObject {
		return nil
	}
	if _, ok := def.Fields[ref.Field]; !ok {
		return nil
	}

	schema, ctx, err := loadCUEValueSchema(root, spec, spec.definition+".fields."+ref.Field+".value")
	if err != nil {
		return err
	}
	return validateCUECandidates(schema, cueCandidateValues(ctx, raw))
}

func cueBuiltInTypeForID(typeID model.TypeID) (cueBuiltInType, bool) {
	spec, ok := cueBuiltInTypesByID[typeID]
	return spec, ok
}

var cueBuiltInTypesByID = map[model.TypeID]cueBuiltInType{
	model.TypeCoreOpaque: {
		importPath: "./types/core/opaque",
		definition: "#Opaque",
		name:       "opaque",
	},
	model.TypeCorePlain: {
		importPath: "./types/core/plain",
		definition: "#Plain",
		name:       "plain",
	},
	model.TypeCoreSecret: {
		importPath: "./types/core/secret",
		definition: "#Secret",
		name:       "secret",
	},
	model.TypeCoreURL: {
		importPath: "./types/core/url",
		definition: "#URL",
		name:       "url",
	},
	model.TypeUniverseRedis: {
		importPath: "./types/universe/redis",
		definition: "#Redis",
		name:       "redis",
	},
}

func cueCandidateValues(ctx *cue.Context, raw string) []cue.Value {
	compiled := ctx.CompileString(raw)
	if compiled.Err() == nil {
		return []cue.Value{compiled, ctx.Encode(raw)}
	}
	return []cue.Value{ctx.Encode(raw)}
}

func validateCUECandidates(schema cue.Value, candidates []cue.Value) error {
	var lastErr error
	for _, candidate := range candidates {
		if err := candidate.Err(); err != nil {
			lastErr = err
			continue
		}
		if err := schema.Unify(candidate).Validate(cue.Concrete(true)); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func loadCUEValueSchema(root string, spec cueBuiltInType, path string) (cue.Value, *cue.Context, error) {
	if root == "" {
		return cue.Value{}, nil, fmt.Errorf("cue root is not configured")
	}

	cfg := &load.Config{Dir: filepath.Clean(root)}
	instances := load.Instances([]string{spec.importPath}, cfg)
	if len(instances) == 0 {
		return cue.Value{}, nil, fmt.Errorf("cue type %s: no instances loaded", spec.importPath)
	}
	if err := instances[0].Err; err != nil {
		return cue.Value{}, nil, fmt.Errorf("cue type %s: %w", spec.importPath, err)
	}

	ctx := cuecontext.New()
	value := ctx.BuildInstance(instances[0])
	if err := value.Err(); err != nil {
		return cue.Value{}, nil, fmt.Errorf("cue type %s: %w", spec.importPath, err)
	}

	schema := value.LookupPath(cue.ParsePath(path))
	if err := schema.Err(); err != nil {
		return cue.Value{}, nil, fmt.Errorf("cue type %s %s: %w", spec.importPath, path, err)
	}
	return schema, ctx, nil
}
