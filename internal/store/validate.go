package store

import (
	"fmt"

	"github.com/runmedev/owl/internal/model"
	"github.com/runmedev/owl/internal/registry"
)

func ValidateState(state model.EffectiveState, types registry.TypeProvider) []model.Diagnostic {
	if types == nil {
		types = registry.NewBuiltInRegistry()
	}

	var diagnostics []model.Diagnostic
	for ref, value := range state.Values {
		if value.FieldRef != ref {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "state.field-ref-mismatch",
				Message:  "value field ref does not match map key",
				FieldRef: ref,
			})
		}

		def, ok := types.ResolveType(ref.TypeID)
		if !ok {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "state.unknown-type",
				Message:  fmt.Sprintf("unknown type %q", ref.TypeID),
				FieldRef: ref,
			})
			continue
		}

		if def.Kind == model.FieldKindObject {
			field, ok := def.Fields[ref.Field]
			if !ok {
				diagnostics = append(diagnostics, model.Diagnostic{
					Severity: model.DiagnosticError,
					Code:     "state.unknown-field",
					Message:  fmt.Sprintf("unknown field %q on type %s", ref.Field, ref.TypeID.Alias()),
					FieldRef: ref,
				})
				continue
			}
			if value.Sensitivity != "" && field.Sensitivity != "" && value.Sensitivity != field.Sensitivity {
				diagnostics = append(diagnostics, model.Diagnostic{
					Severity: model.DiagnosticWarning,
					Code:     "state.sensitivity-mismatch",
					Message:  fmt.Sprintf("field sensitivity is %s but value sensitivity is %s", field.Sensitivity, value.Sensitivity),
					FieldRef: ref,
				})
			}
		}
	}

	seenBindings := make(map[string]model.FieldRef)
	for _, binding := range state.Bindings {
		if binding.FieldRef.TypeID == "" {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "state.binding-missing-field",
				Message:  "binding has no field ref",
				Key:      string(binding.Key),
			})
			continue
		}
		if _, ok := types.ResolveType(binding.FieldRef.TypeID); !ok {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "state.binding-unknown-type",
				Message:  fmt.Sprintf("binding references unknown type %q", binding.FieldRef.TypeID),
				Key:      string(binding.Key),
				FieldRef: binding.FieldRef,
			})
		}
		if existing, ok := seenBindings[string(binding.Key)]; ok && existing != binding.FieldRef {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticWarning,
				Code:     "state.binding-conflict",
				Message:  "projection key is bound to multiple fields",
				Key:      string(binding.Key),
				FieldRef: binding.FieldRef,
			})
			continue
		}
		seenBindings[string(binding.Key)] = binding.FieldRef
	}

	return diagnostics
}
