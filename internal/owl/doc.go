// Package owl retains the original programmable graph/query runtime.
//
// The v2 store lifecycle now lives in internal/store and owns typed state
// semantics. Keep this package available as the frontend-agnostic query/runtime
// boundary while that runtime is adapted to read from the v2 model.
package owl
