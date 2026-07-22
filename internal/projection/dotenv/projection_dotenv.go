package dotenv

import (
	"sort"
	"strings"
	"time"

	"github.com/runmedev/owl/internal/model"
)

type (
	Source               = model.Source
	Clock                = model.Clock
	OperationIDGenerator = model.OperationIDGenerator
	FieldRef             = model.FieldRef
	ProjectionKey        = model.ProjectionKey
	Sensitivity          = model.Sensitivity
	Exposure             = model.Exposure
	EffectiveState       = model.EffectiveState
	BindingConfidence    = model.BindingConfidence
	Diagnostic           = model.Diagnostic
	Value                = model.Value
	Visibility           = model.Visibility
	Binding              = model.Binding
	RenderPolicy         = model.RenderPolicy
	RenderedVariable     = model.RenderedVariable
	RenderedProjection   = model.RenderedProjection
	OperationID          = model.OperationID
	OperationMetadata    = model.OperationMetadata
)

const (
	BindingConfidenceExplicit    = model.BindingConfidenceExplicit
	BindingConfidenceTypeDerived = model.BindingConfidenceTypeDerived
	BindingConfidenceOpaque      = model.BindingConfidenceOpaque
	DiagnosticInfo               = model.DiagnosticInfo
	DiagnosticError              = model.DiagnosticError
	DiagnosticWarning            = model.DiagnosticWarning
	OperationKindLoad            = model.OperationKindLoad
	OperationKindNormalize       = model.OperationKindNormalize
	ProjectionDotenv             = model.ProjectionDotenv
	ExposureKnown                = model.ExposureKnown
	ExposureOpaque               = model.ExposureOpaque
	SensitivityNonSensitive      = model.SensitivityNonSensitive
	SensitivitySensitive         = model.SensitivitySensitive
	SensitivityUnknown           = model.SensitivityUnknown
	TypeCoreHost                 = model.TypeCoreHost
	TypeCoreOpaque               = model.TypeCoreOpaque
	TypeCorePlain                = model.TypeCorePlain
	TypeCorePort                 = model.TypeCorePort
	TypeCoreSecret               = model.TypeCoreSecret
	TypeCoreURL                  = model.TypeCoreURL
	TypeUniverseRedis            = model.TypeUniverseRedis
	VisibilityHidden             = model.VisibilityHidden
	VisibilityLiteral            = model.VisibilityLiteral
	VisibilityMasked             = model.VisibilityMasked
	VisibilityUnresolved         = model.VisibilityUnresolved
)

var (
	NewEffectiveState                = model.NewEffectiveState
	NewMonotonicOperationIDGenerator = model.NewMonotonicOperationIDGenerator
	RealClock                        = model.RealClock
)

type DotenvIngestOptions struct {
	Source       Source
	Actor        string
	Clock        Clock
	OperationIDs OperationIDGenerator
	Declarations []FieldDeclaration
}

type FieldDeclaration struct {
	FieldRef    FieldRef
	Key         ProjectionKey
	Required    bool
	Description string
	Source      Source
	Sensitivity Sensitivity
	Exposure    Exposure
	UnknownType string
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
	declarationsByKey := make(map[string]FieldDeclaration, len(opts.Declarations))
	declarationKeys := make([]string, 0, len(opts.Declarations))
	for _, declaration := range opts.Declarations {
		key := string(declaration.Key)
		if key == "" {
			key = preferredDotenvKey(declaration.FieldRef)
			declaration.Key = ProjectionKey(key)
		}
		if declaration.Source.Name == "" {
			declaration.Source = Source{Name: ".env.example", Kind: "dotenv-spec"}
		}
		declarationsByKey[key] = declaration
		declarationKeys = append(declarationKeys, key)
	}
	sort.Strings(declarationKeys)

	seenKeys := make(map[string]struct{}, len(values))
	seenFields := make(map[FieldRef]string, len(values)+len(opts.Declarations))
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
		preserveKey := confidence == BindingConfidenceOpaque
		description := ""
		if declaration, ok := declarationsByKey[key]; ok {
			fieldRef = declaration.FieldRef
			confidence = BindingConfidenceExplicit
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
			state.Diagnostics = append(state.Diagnostics, Diagnostic{
				Severity: DiagnosticWarning,
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
		state.Values[fieldRef] = Value{
			FieldRef:        fieldRef,
			Original:        value,
			Resolved:        value,
			Visibility:      VisibilityLiteral,
			Sensitivity:     sensitivity,
			Exposure:        exposure,
			Origin:          origin,
			Source:          source,
			CreatedAt:       now,
			UpdatedAt:       now,
			LastOperationID: opID,
		}
		state.Bindings = append(state.Bindings, newBinding(opID, key, fieldRef, description, source, origin, confidence, explicit, preserveKey, now))
		state.Operations = append(state.Operations, OperationMetadata{
			ID:           opID,
			Kind:         OperationKindLoad,
			Timestamp:    now,
			Actor:        opts.Actor,
			Source:       source,
			ProjectionID: ProjectionDotenv,
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
			state.Diagnostics = append(state.Diagnostics, Diagnostic{
				Severity: DiagnosticWarning,
				Code:     "dotenv.declaration-collision",
				Message:  "dotenv declarations project to the same semantic field; keeping the first field value",
				Key:      key,
				FieldRef: fieldRef,
			})
			continue
		}
		seenFields[fieldRef] = key

		state.Values[fieldRef] = Value{
			FieldRef:        fieldRef,
			Visibility:      VisibilityUnresolved,
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
			BindingConfidenceExplicit,
			true,
			false,
			now,
		))
		state.Operations = append(state.Operations, OperationMetadata{
			ID:           opID,
			Kind:         OperationKindNormalize,
			Timestamp:    now,
			Actor:        opts.Actor,
			Source:       declaration.Source,
			ProjectionID: ProjectionDotenv,
		})
		if declaration.Required {
			state.Diagnostics = append(state.Diagnostics, Diagnostic{
				Severity: DiagnosticError,
				Code:     "dotenv.unresolved-required",
				Message:  "required declared dotenv field has no observed value",
				Key:      key,
				FieldRef: fieldRef,
			})
		}
	}

	return state
}

func RenderDotenv(state EffectiveState, policy RenderPolicy) []RenderedVariable {
	return RenderDotenvProjection(state, policy).Variables
}

func RenderDotenvProjection(state EffectiveState, policy RenderPolicy) RenderedProjection {
	rendered := make([]RenderedVariable, 0, len(state.Bindings))
	diagnostics := make([]Diagnostic, 0)
	keys := make(map[string]FieldRef, len(state.Bindings))
	for _, binding := range state.Bindings {
		value := state.Values[binding.FieldRef]
		key := renderKey(binding, value)
		if value.Visibility == VisibilityUnresolved {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: DiagnosticInfo,
				Code:     "dotenv.render-unresolved",
				Message:  "unresolved semantic field has no dotenv value to render",
				Key:      key,
				FieldRef: binding.FieldRef,
			})
			continue
		}
		if existingRef, ok := keys[key]; ok && existingRef != binding.FieldRef {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: DiagnosticWarning,
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
			case SensitivitySensitive:
				renderValue = "[masked]"
				visibility = VisibilityMasked
			case SensitivityUnknown:
				renderValue = "[hidden]"
				visibility = VisibilityHidden
			}
		}
		rendered = append(rendered, RenderedVariable{
			Key:        key,
			Value:      renderValue,
			Visibility: visibility,
		})
	}
	sort.SliceStable(rendered, func(i, j int) bool {
		return rendered[i].Key < rendered[j].Key
	})
	return RenderedProjection{Variables: rendered, Diagnostics: diagnostics}
}

func newBinding(
	opID OperationID,
	key string,
	fieldRef FieldRef,
	description string,
	source Source,
	origin Source,
	confidence BindingConfidence,
	explicit bool,
	preserveKey bool,
	now time.Time,
) Binding {
	return Binding{
		ID:              string(opID) + ":" + key,
		FieldRef:        fieldRef,
		ProjectionID:    ProjectionDotenv,
		Key:             ProjectionKey(key),
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

func renderKey(binding Binding, value Value) string {
	if binding.PreserveKey || binding.Key != "" {
		return string(binding.Key)
	}
	return preferredDotenvKey(value.FieldRef)
}

func preferredDotenvKey(ref FieldRef) string {
	if ref.TypeID == TypeUniverseRedis {
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

func sensitivityForField(ref FieldRef) Sensitivity {
	if ref.TypeID == TypeUniverseRedis && ref.Field == "password" {
		return SensitivitySensitive
	}
	if ref.TypeID == TypeCoreSecret {
		return SensitivitySensitive
	}
	if ref.TypeID == TypeCorePlain || ref.TypeID == TypeCoreURL || ref.TypeID == TypeCoreHost || ref.TypeID == TypeCorePort {
		return SensitivityNonSensitive
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

func exposureForField(ref FieldRef) Exposure {
	if ref.TypeID == TypeCoreOpaque {
		return ExposureOpaque
	}
	return ExposureKnown
}

func declarationSensitivity(declaration FieldDeclaration) Sensitivity {
	if declaration.Sensitivity != "" {
		return declaration.Sensitivity
	}
	return sensitivityForField(declaration.FieldRef)
}

func declarationExposure(declaration FieldDeclaration) Exposure {
	if declaration.Exposure != "" {
		return declaration.Exposure
	}
	return exposureForField(declaration.FieldRef)
}
