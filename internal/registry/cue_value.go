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
	if !ok || spec.valueDefinition == "" {
		return nil
	}
	if root == "" {
		return fmt.Errorf("cue root is not configured")
	}

	cfg := &load.Config{Dir: filepath.Clean(root)}
	instances := load.Instances([]string{spec.importPath}, cfg)
	if len(instances) == 0 {
		return fmt.Errorf("cue type %s: no instances loaded", spec.importPath)
	}
	if err := instances[0].Err; err != nil {
		return fmt.Errorf("cue type %s: %w", spec.importPath, err)
	}

	ctx := cuecontext.New()
	value := ctx.BuildInstance(instances[0])
	if err := value.Err(); err != nil {
		return fmt.Errorf("cue type %s: %w", spec.importPath, err)
	}

	schema := value.LookupPath(cue.ParsePath(spec.valueDefinition))
	if err := schema.Err(); err != nil {
		return fmt.Errorf("cue type %s %s: %w", spec.importPath, spec.valueDefinition, err)
	}

	candidate := cueCandidateValue(ctx, typeID, raw)
	if err := candidate.Err(); err != nil {
		return err
	}
	return schema.Unify(candidate).Validate(cue.Concrete(true))
}

func cueBuiltInTypeForID(typeID model.TypeID) (cueBuiltInType, bool) {
	spec, ok := cueBuiltInTypesByID[typeID]
	return spec, ok
}

var cueBuiltInTypesByID = map[model.TypeID]cueBuiltInType{
	model.TypeCoreOpaque: {
		importPath:      "./types/core/opaque",
		definition:      "#Opaque",
		valueDefinition: "#OpaqueValue",
		name:            "opaque",
	},
	model.TypeCorePlain: {
		importPath:      "./types/core/plain",
		definition:      "#Plain",
		valueDefinition: "#PlainValue",
		name:            "plain",
	},
	model.TypeCoreSecret: {
		importPath:      "./types/core/secret",
		definition:      "#Secret",
		valueDefinition: "#SecretValue",
		name:            "secret",
	},
	model.TypeCoreURL: {
		importPath:      "./types/core/url",
		definition:      "#URL",
		valueDefinition: "#URLValue",
		name:            "url",
	},
	model.TypeCoreHost: {
		importPath:      "./types/core/host",
		definition:      "#Host",
		valueDefinition: "#HostValue",
		name:            "host",
	},
	model.TypeCorePort: {
		importPath:      "./types/core/port",
		definition:      "#Port",
		valueDefinition: "#PortValue",
		name:            "port",
	},
	model.TypeUniverseRedis: {
		importPath:      "./types/universe/redis",
		definition:      "#Redis",
		valueDefinition: "#RedisValue",
		name:            "redis",
	},
}

func cueCandidateValue(ctx *cue.Context, typeID model.TypeID, raw string) cue.Value {
	if typeID == model.TypeCorePort {
		return ctx.CompileString(raw)
	}
	return ctx.Encode(raw)
}
