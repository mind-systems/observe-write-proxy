// Package webassets embeds the admin GUI's static assets into the binary.
// The embed directive must live here, at the module root: //go:embed
// interprets its pattern relative to the source file's directory and
// forbids "..", and web/ is fixed at the module root by ARCHITECTURE.md.
// internal/admin still owns serving it — this file only holds the raw
// embed.FS, which internal/admin roots at "web" via fs.Sub.
package webassets

import "embed"

//go:embed web
var FS embed.FS
