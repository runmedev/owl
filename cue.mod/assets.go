package cuemod

import "embed"

// BuiltInFS contains the module metadata for Owl's embedded CUE catalog.
//
//go:embed *.cue
var BuiltInFS embed.FS
