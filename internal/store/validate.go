package store

import (
	"fmt"
	"strconv"
	"strings"

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
				Owner:    model.DiagnosticOwnerValidation,
			})
		}

		def, ok := types.ResolveType(ref.TypeID)
		if !ok {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "state.unknown-type",
				Message:  fmt.Sprintf("unknown type %q", ref.TypeID),
				FieldRef: ref,
				Owner:    model.DiagnosticOwnerValidation,
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
					Owner:    model.DiagnosticOwnerValidation,
				})
				continue
			}
			if value.Sensitivity != "" && field.Sensitivity != "" && value.Sensitivity != field.Sensitivity {
				diagnostics = append(diagnostics, model.Diagnostic{
					Severity: model.DiagnosticWarning,
					Code:     "state.sensitivity-mismatch",
					Message:  fmt.Sprintf("field sensitivity is %s but value sensitivity is %s", field.Sensitivity, value.Sensitivity),
					FieldRef: ref,
					Owner:    model.DiagnosticOwnerValidation,
				})
			}
			diagnostics = append(diagnostics, validatePrimitiveValue(field.TypeID, value)...)
			continue
		}

		diagnostics = append(diagnostics, validatePrimitiveValue(def.ID, value)...)
	}

	seenBindings := make(map[string]model.FieldRef)
	for _, binding := range state.Bindings {
		if binding.FieldRef.TypeID == "" {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "state.binding-missing-field",
				Message:  "binding has no field ref",
				Key:      string(binding.Key),
				Owner:    model.DiagnosticOwnerValidation,
			})
			continue
		}
		if _, ok := types.ResolveType(binding.FieldRef.TypeID); !ok {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "contract.unknown-type",
				Message:  fmt.Sprintf("binding references unknown type %q", binding.FieldRef.TypeID),
				Key:      string(binding.Key),
				FieldRef: binding.FieldRef,
				Owner:    model.DiagnosticOwnerValidation,
			})
		}
		if existing, ok := seenBindings[string(binding.Key)]; ok && existing != binding.FieldRef {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "contract.binding-conflict",
				Message:  "projection key is bound to multiple fields",
				Key:      string(binding.Key),
				FieldRef: binding.FieldRef,
				Owner:    model.DiagnosticOwnerValidation,
			})
			continue
		}
		seenBindings[string(binding.Key)] = binding.FieldRef
		value := state.Values[binding.FieldRef]
		if binding.Required && value.Visibility == model.VisibilityUnresolved {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "dotenv.unresolved-required",
				Message:  "required declared dotenv field has no observed value",
				Key:      string(binding.Key),
				FieldRef: binding.FieldRef,
				Owner:    model.DiagnosticOwnerValidation,
			})
		}
	}

	return diagnostics
}

func validatePrimitiveValue(typeID model.TypeID, value model.Value) []model.Diagnostic {
	if value.Visibility == model.VisibilityUnresolved {
		return nil
	}

	switch typeID {
	case model.TypeCoreHost:
		if strings.TrimSpace(value.Resolved) == "" {
			return []model.Diagnostic{{
				Severity: model.DiagnosticError,
				Code:     "type.empty-host",
				Message:  "host value must not be empty",
				Key:      "",
				FieldRef: value.FieldRef,
				Owner:    model.DiagnosticOwnerValidation,
			}}
		}
	case model.TypeCorePort:
		port, err := strconv.Atoi(value.Resolved)
		if err != nil || port < 1 || port > 65535 {
			return []model.Diagnostic{{
				Severity: model.DiagnosticError,
				Code:     "type.invalid-port",
				Message:  "port value must be an integer between 1 and 65535",
				Key:      "",
				FieldRef: value.FieldRef,
				Owner:    model.DiagnosticOwnerValidation,
			}}
		}
	case model.TypeCoreSecret:
		if strings.TrimSpace(value.Resolved) == "" {
			return []model.Diagnostic{{
				Severity: model.DiagnosticError,
				Code:     "type.empty-secret",
				Message:  "secret value must not be empty",
				FieldRef: value.FieldRef,
				Owner:    model.DiagnosticOwnerValidation,
			}}
		}
	}
	return nil
}
