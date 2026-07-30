package registry

import (
	"cuelang.org/go/cue"

	"github.com/runmedev/owl/internal/model"
)

func ValidateCUEValue(root string, typeID model.TypeID, raw string) error {
	catalog, err := newCUECatalog(root)
	if err != nil {
		return err
	}
	return catalog.ValidateValue(typeID, raw)
}

func ValidateCUEFieldValue(root string, types map[model.TypeID]model.TypeDef, ref model.FieldRef, raw string) error {
	catalog, err := newCUECatalog(root)
	if err != nil {
		return err
	}
	return catalog.ValidateFieldValue(types, ref, raw)
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
