package frontend

import "embed"

// Assets contains the production frontend bundle.
//
//go:embed all:dist
var Assets embed.FS
