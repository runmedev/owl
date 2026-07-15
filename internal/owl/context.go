package owl

import (
	"context"
)

type executionInfoKey struct{}

// ExecutionInfo describes the Runme execution context that produced an env update.
// It is intentionally small so Owl can stay independent of Runme's runner packages.
type ExecutionInfo struct {
	KnownID     string
	KnownName   string
	ExecContext string
}

func ContextWithExecutionInfo(ctx context.Context, info ExecutionInfo) context.Context {
	return context.WithValue(ctx, executionInfoKey{}, info)
}

func ExecutionInfoFromContext(ctx context.Context) (ExecutionInfo, bool) {
	info, ok := ctx.Value(executionInfoKey{}).(ExecutionInfo)
	return info, ok
}
