package owl

import (
	"context"

	impl "github.com/runmedev/owl/internal/owl"
)

const (
	AtomicNameOpaque   = impl.AtomicNameOpaque
	AtomicNamePlain    = impl.AtomicNamePlain
	AtomicNameSecret   = impl.AtomicNameSecret
	AtomicNamePassword = impl.AtomicNamePassword
	AtomicNameDefault  = impl.AtomicNameDefault

	SpecTypeKey = impl.SpecTypeKey

	ValidateErrorVarRequired      = impl.ValidateErrorVarRequired
	ValidateErrorTagFailed        = impl.ValidateErrorTagFailed
	ValidateErrorResolutionFailed = impl.ValidateErrorResolutionFailed
)

type (
	ExecutionInfo = impl.ExecutionInfo

	Store       = impl.Store
	StoreOption = impl.StoreOption

	SetVar      = impl.SetVar
	SetVarSpec  = impl.SetVarSpec
	SetVarValue = impl.SetVarValue
	SetVarError = impl.SetVarError
	SetVarItem  = impl.SetVarItem
	SetVarItems = impl.SetVarItems

	Spec     = impl.Spec
	Specs    = impl.Specs
	SpecDef  = impl.SpecDef
	SpecDefs = impl.SpecDefs

	ValidationError       = impl.ValidationError
	ValidationErrors      = impl.ValidationErrors
	ValidateErrorType     = impl.ValidateErrorType
	TagFailedError        = impl.TagFailedError
	RequiredError         = impl.RequiredError
	ResolutionFailedError = impl.ResolutionFailedError
)

var (
	NewStore          = impl.NewStore
	WithSpecFile      = impl.WithSpecFile
	WithEnvFile       = impl.WithEnvFile
	WithEnvs          = impl.WithEnvs
	WithResolutionCRD = impl.WithResolutionCRD
	WithSpecDefsCRD   = impl.WithSpecDefsCRD
	WithLogger        = impl.WithLogger

	ParseRawSpec = impl.ParseRawSpec

	NewTagFailedError        = impl.NewTagFailedError
	NewRequiredError         = impl.NewRequiredError
	NewResolutionFailedError = impl.NewResolutionFailedError
)

func ContextWithExecutionInfo(ctx context.Context, info ExecutionInfo) context.Context {
	return impl.ContextWithExecutionInfo(ctx, info)
}

func ExecutionInfoFromContext(ctx context.Context) (ExecutionInfo, bool) {
	return impl.ExecutionInfoFromContext(ctx)
}
