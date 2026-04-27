// Package web embeds the HTML templates and static assets that ship with
// the contribcard binary. Renderers consume them via the exported fs.FS values.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:templates
var templatesFS embed.FS

//go:embed all:static
var staticFS embed.FS

// Templates returns an fs.FS rooted at the templates directory (so callers
// see "_layout.tmpl" etc. rather than "templates/_layout.tmpl").
func Templates() fs.FS {
	sub, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		// Should never happen — directory is embedded above.
		panic(err)
	}
	return sub
}

// Static returns an fs.FS rooted at the static directory (so callers see
// "css/style.css", "js/search.js", "favicon.svg").
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
