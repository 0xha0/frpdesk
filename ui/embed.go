// Package ui holds the built web UI (npm run build in this directory -> dist/), embedded into the
// binary. Without a build, dist/ holds only a placeholder page.
package ui

import "embed"

//go:embed all:dist
var Dist embed.FS
