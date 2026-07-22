package dotenv

import (
	"sort"
	"strings"
	"time"

	"github.com/runmedev/owl/internal/model"
)

type DotenvIngestOptions struct {
	Source       model.Source
	Actor        string
	Clock        model.Clock
	OperationIDs model.OperationIDGenerator
	Declarations []FieldDeclaration
}

type FieldDeclaration struct {
	FieldRef    model.FieldRef
	Key         model.ProjectionKey
	Required    bool
	Description string
	Source      model.Source
	Sensitivity model.Sensitivity
	Exposure    model.Exposure
	UnknownType string
}

func IngestDotenv(values map[string]string, opts DotenvIngestOptions) model.EffectiveState {
	clock := opts.Clock
	if clock == nil {
		clock = model.RealClock
	}
	opIDs := opts.OperationIDs
	if opIDs == nil {
		opIDs = model.NewMonotonicOperationIDGenerator("op")
	}
	source := opts.Source
	if source.Name == "" {
		source = model.Source{Name: ".env", Kind: "dotenv"}
	}

	state := model.NewEffectiveState()
	declarationsByKey := make(map[string]FieldDeclaration, len(opts.Declarations))
	declarationKeys := make([]string, 0, len(opts.Declarations))
	for _, declaration := range opts.Declarations {
		key := string(declaration.Key)
		if key == "" {
			key = preferredDotenvKey(declaration.FieldRef)
			declaration.Key = model.ProjectionKey(key)
		}
		if declaration.Source.Name == "" {
			declaration.Source = model.Source{Name: ".env.example", Kind: "dotenv-spec"}
		}
		declarationsByKey[key] = declaration
		declarationKeys = append(declarationKeys, key)
	}
	sort.Strings(declarationKeys)

	seenKeys := make(map[string]struct{}, len(values))
	seenFields := make(map[model.FieldRef]string, len(values)+len(opts.Declarations))
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
		origin := source
		explicit := false
		preserveKey := confidence == model.BindingConfidenceOpaque
		description := ""
		if declaration, ok := declarationsByKey[key]; ok {
			fieldRef = declaration.FieldRef
			confidence = model.BindingConfidenceExplicit
			diagnostic = nil
			origin = declaration.Source
			explicit = true
			preserveKey = false
			description = declaration.Description
		}
		if diagnostic != nil {
			state.Diagnostics = append(state.Diagnostics, *diagnostic)
		}
		if _, ok := seenFields[fieldRef]; ok {
			state.Diagnostics = append(state.Diagnostics, model.Diagnostic{
				Severity: model.DiagnosticWarning,
				Code:     "dotenv.collision",
				Message:  "dotenv keys project to the same semantic field; keeping the first value",
				Key:      key,
				FieldRef: fieldRef,
			})
			state.Bindings = append(state.Bindings, newBinding(opID, key, fieldRef, description, source, origin, confidence, explicit, preserveKey, now))
			seenKeys[key] = struct{}{}
			continue
		}
		seenFields[fieldRef] = key
		seenKeys[key] = struct{}{}

		sensitivity := sensitivityForField(fieldRef)
		exposure := exposureForField(fieldRef)
		if declaration, ok := declarationsByKey[key]; ok {
			sensitivity = declarationSensitivity(declaration)
			exposure = declarationExposure(declaration)
		}
		state.Values[fieldRef] = model.Value{
			FieldRef:        fieldRef,
			Original:        value,
			Resolved:        value,
			Visibility:      model.VisibilityLiteral,
			Sensitivity:     sensitivity,
			Exposure:        exposure,
			Origin:          origin,
			Source:          source,
			CreatedAt:       now,
			UpdatedAt:       now,
			LastOperationID: opID,
		}
		state.Bindings = append(state.Bindings, newBinding(opID, key, fieldRef, description, source, origin, confidence, explicit, preserveKey, now))
		state.Operations = append(state.Operations, model.OperationMetadata{
			ID:           opID,
			Kind:         model.OperationKindLoad,
			Timestamp:    now,
			Actor:        opts.Actor,
			Source:       source,
			ProjectionID: model.ProjectionDotenv,
		})
	}

	for _, key := range declarationKeys {
		if _, ok := seenKeys[key]; ok {
			continue
		}
		declaration := declarationsByKey[key]
		now := clock()
		opID := opIDs()
		fieldRef := declaration.FieldRef
		if _, ok := seenFields[fieldRef]; ok {
			state.Diagnostics = append(state.Diagnostics, model.Diagnostic{
				Severity: model.DiagnosticWarning,
				Code:     "dotenv.declaration-collision",
				Message:  "dotenv declarations project to the same semantic field; keeping the first field value",
				Key:      key,
				FieldRef: fieldRef,
			})
			continue
		}
		seenFields[fieldRef] = key

		state.Values[fieldRef] = model.Value{
			FieldRef:        fieldRef,
			Visibility:      model.VisibilityUnresolved,
			Sensitivity:     declarationSensitivity(declaration),
			Exposure:        declarationExposure(declaration),
			Origin:          declaration.Source,
			Source:          declaration.Source,
			CreatedAt:       now,
			UpdatedAt:       now,
			LastOperationID: opID,
		}
		state.Bindings = append(state.Bindings, newBinding(
			opID,
			key,
			fieldRef,
			declaration.Description,
			declaration.Source,
			declaration.Source,
			model.BindingConfidenceExplicit,
			true,
			false,
			now,
		))
		state.Operations = append(state.Operations, model.OperationMetadata{
			ID:           opID,
			Kind:         model.OperationKindNormalize,
			Timestamp:    now,
			Actor:        opts.Actor,
			Source:       declaration.Source,
			ProjectionID: model.ProjectionDotenv,
		})
		if declaration.Required {
			state.Diagnostics = append(state.Diagnostics, model.Diagnostic{
				Severity: model.DiagnosticError,
				Code:     "dotenv.unresolved-required",
				Message:  "required declared dotenv field has no observed value",
				Key:      key,
				FieldRef: fieldRef,
			})
		}
	}

	return state
}

func RenderDotenv(state model.EffectiveState, policy model.RenderPolicy) []model.RenderedVariable {
	return RenderDotenvProjection(state, policy).Variables
}

func RenderDotenvProjection(state model.EffectiveState, policy model.RenderPolicy) model.RenderedProjection {
	rendered := make([]model.RenderedVariable, 0, len(state.Bindings))
	diagnostics := make([]model.Diagnostic, 0)
	keys := make(map[string]model.FieldRef, len(state.Bindings))
	for _, binding := range state.Bindings {
		value := state.Values[binding.FieldRef]
		key := renderKey(binding, value)
		if value.Visibility == model.VisibilityUnresolved {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticInfo,
				Code:     "dotenv.render-unresolved",
				Message:  "unresolved semantic field has no dotenv value to render",
				Key:      key,
				FieldRef: binding.FieldRef,
			})
			continue
		}
		if existingRef, ok := keys[key]; ok && existingRef != binding.FieldRef {
			diagnostics = append(diagnostics, model.Diagnostic{
				Severity: model.DiagnosticWarning,
				Code:     "dotenv.render-collision",
				Message:  "multiple semantic fields render to the same dotenv key; skipping later value",
				Key:      key,
				FieldRef: binding.FieldRef,
			})
			continue
		}
		keys[key] = binding.FieldRef

		renderValue := value.Resolved
		visibility := value.Visibility
		if !policy.Insecure {
			switch value.Sensitivity {
			case model.SensitivitySensitive:
				renderValue = "[masked]"
				visibility = model.VisibilityMasked
			case model.SensitivityUnknown:
				renderValue = "[hidden]"
				visibility = model.VisibilityHidden
			}
		}
		rendered = append(rendered, model.RenderedVariable{
			Key:        key,
			Value:      renderValue,
			Visibility: visibility,
		})
	}
	sort.SliceStable(rendered, func(i, j int) bool {
		return rendered[i].Key < rendered[j].Key
	})
	return model.RenderedProjection{Variables: rendered, Diagnostics: diagnostics}
}

func newBinding(
	opID model.OperationID,
	key string,
	fieldRef model.FieldRef,
	description string,
	source model.Source,
	origin model.Source,
	confidence model.BindingConfidence,
	explicit bool,
	preserveKey bool,
	now time.Time,
) model.Binding {
	return model.Binding{
		ID:              string(opID) + ":" + key,
		FieldRef:        fieldRef,
		ProjectionID:    model.ProjectionDotenv,
		Key:             model.ProjectionKey(key),
		Description:     description,
		Source:          source,
		Origin:          origin,
		Confidence:      confidence,
		Explicit:        explicit,
		PreserveKey:     preserveKey,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastOperationID: opID,
	}
}

func renderKey(binding model.Binding, value model.Value) string {
	if binding.PreserveKey || binding.Key != "" {
		return string(binding.Key)
	}
	return preferredDotenvKey(value.FieldRef)
}

func preferredDotenvKey(ref model.FieldRef) string {
	if ref.TypeID == model.TypeUniverseRedis {
		field, ok := redisPreferredSuffix(ref.Field)
		if ok {
			if ref.Instance == "" || ref.Instance == "default" {
				return "REDIS_" + field
			}
			return strings.ToUpper(strings.ReplaceAll(ref.Instance, ".", "_")) + "_REDIS_" + field
		}
	}
	return strings.ToUpper(strings.ReplaceAll(ref.Field, ".", "_"))
}

func dotenvFieldRef(key string) (model.FieldRef, model.BindingConfidence, *model.Diagnostic) {
	parts := strings.Split(key, "_")
	if len(parts) >= 2 && parts[len(parts)-2] == "REDIS" {
		field, ok := redisField(parts[len(parts)-1])
		if ok {
			instance := "default"
			if len(parts) > 2 {
				instance = strings.ToLower(strings.Join(parts[:len(parts)-2], "_"))
			}
			return model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: instance, Field: field}, model.BindingConfidenceTypeDerived, nil
		}
	}
	if strings.HasPrefix(key, "REDIS_") {
		field, ok := redisField(strings.TrimPrefix(key, "REDIS_"))
		if ok {
			return model.FieldRef{TypeID: model.TypeUniverseRedis, Instance: "default", Field: field}, model.BindingConfidenceTypeDerived, nil
		}
	}

	ref := model.FieldRef{TypeID: model.TypeCoreOpaque, Instance: "default", Field: opaqueFieldName(key)}
	diagnostic := &model.Diagnostic{
		Severity: model.DiagnosticInfo,
		Code:     "dotenv.opaque",
		Message:  "dotenv key has no explicit type declaration and remains core/opaque",
		Key:      key,
		FieldRef: ref,
	}
	return ref, model.BindingConfidenceOpaque, diagnostic
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

func redisPreferredSuffix(field string) (string, bool) {
	switch field {
	case "host":
		return "HOST", true
	case "port":
		return "PORT", true
	case "password":
		return "PASSWORD", true
	default:
		return "", false
	}
}

func opaqueFieldName(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "."))
}

func sensitivityForField(ref model.FieldRef) model.Sensitivity {
	if ref.TypeID == model.TypeUniverseRedis && ref.Field == "password" {
		return model.SensitivitySensitive
	}
	if ref.TypeID == model.TypeCoreSecret {
		return model.SensitivitySensitive
	}
	if ref.TypeID == model.TypeCorePlain || ref.TypeID == model.TypeCoreURL || ref.TypeID == model.TypeCoreHost || ref.TypeID == model.TypeCorePort {
		return model.SensitivityNonSensitive
	}
	if ref.TypeID == model.TypeCoreOpaque {
		key := strings.ToUpper(ref.Field)
		switch {
		case strings.Contains(key, "PASSWORD"),
			strings.Contains(key, "SECRET"),
			strings.Contains(key, "TOKEN"),
			strings.Contains(key, "API.KEY"),
			strings.Contains(key, "PRIVATE.KEY"):
			return model.SensitivitySensitive
		default:
			return model.SensitivityUnknown
		}
	}
	return model.SensitivityNonSensitive
}

func exposureForField(ref model.FieldRef) model.Exposure {
	if ref.TypeID == model.TypeCoreOpaque {
		return model.ExposureOpaque
	}
	return model.ExposureClear
}

func declarationSensitivity(declaration FieldDeclaration) model.Sensitivity {
	if declaration.Sensitivity != "" {
		return declaration.Sensitivity
	}
	return sensitivityForField(declaration.FieldRef)
}

func declarationExposure(declaration FieldDeclaration) model.Exposure {
	if declaration.Exposure != "" {
		return declaration.Exposure
	}
	return exposureForField(declaration.FieldRef)
}
