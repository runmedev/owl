package types

import "embed"

// BuiltInFS contains Owl's built-in core and universe CUE type definitions.
//
//go:embed core universe
var BuiltInFS embed.FS
