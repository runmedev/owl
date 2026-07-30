package store

import (
	"fmt"
	"strings"

	cueerrors "cuelang.org/go/cue/errors"

	"github.com/runmedev/owl/internal/model"
)

func invalidTypeDiagnostic(code string, name string, value model.Value, err error) []model.Diagnostic {
	details := cueDiagnosticDetails(err)
	return []model.Diagnostic{{
		Severity: model.DiagnosticError,
		Code:     code,
		Message:  fmt.Sprintf("%s %s is invalid: %s", name, diagnosticValueLabel(value), friendlyDiagnosticDetail(details)),
		Details:  details,
		Key:      "",
		FieldRef: value.FieldRef,
		Owner:    model.DiagnosticOwnerValidation,
	}}
}

func diagnosticValueLabel(value model.Value) string {
	if value.Sensitivity == model.SensitivitySensitive || value.Visibility == model.VisibilityMasked || value.Visibility == model.VisibilityHidden {
		return "value"
	}
	return fmt.Sprintf("value %q", value.Resolved)
}

func cueDiagnosticDetails(err error) []string {
	if err == nil {
		return []string{"unknown CUE error"}
	}
	detail := strings.TrimSpace(cueerrors.Details(err, nil))
	if detail == "" {
		detail = err.Error()
	}
	lines := strings.Split(detail, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case strings.Contains(line, ".cue:"):
			continue
		case strings.Contains(line, "errors in empty disjunction"):
			continue
		default:
			if _, after, ok := strings.Cut(line, ": "); ok {
				line = after
			}
			line = strings.TrimSuffix(line, ":")
			filtered = append(filtered, strings.Join(strings.Fields(line), " "))
		}
	}
	if len(filtered) == 0 {
		return []string{strings.Join(strings.Fields(detail), " ")}
	}
	return filtered
}

func friendlyDiagnosticDetail(details []string) string {
	return strings.Join(details, "; ")
}

func typeNameForDiagnostic(typeID model.TypeID) string {
	alias := typeID.Alias()
	if _, name, ok := strings.Cut(alias, "/"); ok {
		return name
	}
	return alias
}
