package frontend

import "embed"

// Assets contains the production frontend bundle from frontend/dist, generated
// by `npm --prefix frontend run build` and gitignored. dist must exist for this
// package to compile, so fresh clones need one frontend build before `go build`
// (scripts/dev.sh does this automatically). In `wails dev` assets are served
// from the Vite dev server instead; the embedded copy is only used in
// production builds.
//
//go:embed all:dist
var Assets embed.FS
