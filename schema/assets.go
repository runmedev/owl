package schema

import "embed"

// BuiltInFS contains the CUE schema shared by Owl's built-in types.
//
//go:embed *.cue
var BuiltInFS embed.FS
