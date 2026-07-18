package owl

import (
	"sort"
	"strings"
)

type DotenvIngestOptions struct {
	Source       Source
	Actor        string
	Clock        Clock
	OperationIDs OperationIDGenerator
}

func IngestDotenv(values map[string]string, opts DotenvIngestOptions) EffectiveState {
	clock := opts.Clock
	if clock == nil {
		clock = RealClock
	}
	opIDs := opts.OperationIDs
	if opIDs == nil {
		opIDs = NewMonotonicOperationIDGenerator("op")
	}
	source := opts.Source
	if source.Name == "" {
		source = Source{Name: ".env", Kind: "dotenv"}
	}

	state := NewEffectiveState()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		now := clock()
		opID := opIDs()
		value := values[key]
		fieldRef, confidence, diagnostic := dotenvFieldRef(key)
		if diagnostic != nil {
			state.Diagnostics = append(state.Diagnostics, *diagnostic)
		}

		sensitivity := sensitivityForField(fieldRef)
		visibility := visibilityForField(fieldRef)
		state.Values[fieldRef] = Value{
			FieldRef:           fieldRef,
			Original:           value,
			Resolved:           value,
			Status:             ValueStatusLiteral,
			Sensitivity:        sensitivity,
			SemanticVisibility: visibility,
			Origin:             source,
			Source:             source,
			CreatedAt:          now,
			UpdatedAt:          now,
			LastOperationID:    opID,
		}
		state.Bindings = append(state.Bindings, Binding{
			ID:              string(opID) + ":" + key,
			FieldRef:        fieldRef,
			ProjectionID:    ProjectionDotenv,
			Key:             ProjectionKey(key),
			Source:          source,
			Origin:          source,
			Confidence:      confidence,
			PreserveKey:     confidence == BindingConfidenceOpaque,
			CreatedAt:       now,
			UpdatedAt:       now,
			LastOperationID: opID,
		})
	}

	return state
}

func RenderDotenv(state EffectiveState, policy RenderPolicy) []RenderedVariable {
	rendered := make([]RenderedVariable, 0, len(state.Bindings))
	for _, binding := range state.Bindings {
		value := state.Values[binding.FieldRef]
		renderValue := value.Resolved
		status := value.Status
		if !policy.Insecure {
			switch value.Sensitivity {
			case SensitivitySensitive:
				renderValue = "[masked]"
				status = ValueStatusMasked
			case SensitivityUnknown:
				renderValue = "[hidden]"
				status = ValueStatusHidden
			}
		}
		rendered = append(rendered, RenderedVariable{
			Key:    string(binding.Key),
			Value:  renderValue,
			Status: status,
		})
	}
	sort.SliceStable(rendered, func(i, j int) bool {
		return rendered[i].Key < rendered[j].Key
	})
	return rendered
}

func dotenvFieldRef(key string) (FieldRef, BindingConfidence, *Diagnostic) {
	parts := strings.Split(key, "_")
	if len(parts) >= 2 && parts[len(parts)-2] == "REDIS" {
		field, ok := redisField(parts[len(parts)-1])
		if ok {
			instance := "default"
			if len(parts) > 2 {
				instance = strings.ToLower(strings.Join(parts[:len(parts)-2], "_"))
			}
			return FieldRef{TypeID: TypeUniverseRedis, Instance: instance, Field: field}, BindingConfidenceTypeDerived, nil
		}
	}
	if strings.HasPrefix(key, "REDIS_") {
		field, ok := redisField(strings.TrimPrefix(key, "REDIS_"))
		if ok {
			return FieldRef{TypeID: TypeUniverseRedis, Instance: "default", Field: field}, BindingConfidenceTypeDerived, nil
		}
	}

	ref := FieldRef{TypeID: TypeCoreOpaque, Instance: "default", Field: opaqueFieldName(key)}
	diagnostic := &Diagnostic{
		Severity: DiagnosticInfo,
		Code:     "dotenv.opaque",
		Message:  "dotenv key has no explicit type declaration and remains core/opaque",
		Key:      key,
		FieldRef: ref,
	}
	return ref, BindingConfidenceOpaque, diagnostic
}

func redisField(suffix string) (string, bool) {
	switch suffix {
	case "HOST":
		return "host", true
	case "PORT":
		return "port", true
	case "PASSWORD":
		return "password", true
	default:
		return "", false
	}
}

func opaqueFieldName(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "."))
}

func sensitivityForField(ref FieldRef) Sensitivity {
	if ref.TypeID == TypeUniverseRedis && ref.Field == "password" {
		return SensitivitySensitive
	}
	if ref.TypeID == TypeCoreOpaque {
		key := strings.ToUpper(ref.Field)
		switch {
		case strings.Contains(key, "PASSWORD"),
			strings.Contains(key, "SECRET"),
			strings.Contains(key, "TOKEN"),
			strings.Contains(key, "API.KEY"),
			strings.Contains(key, "PRIVATE.KEY"):
			return SensitivitySensitive
		default:
			return SensitivityUnknown
		}
	}
	return SensitivityNonSensitive
}

func visibilityForField(ref FieldRef) SemanticVisibility {
	if ref.TypeID == TypeCoreOpaque {
		return SemanticVisibilityOpaque
	}
	return SemanticVisibilityKnown
}
