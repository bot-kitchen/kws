package web

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var content embed.FS

// Templates returns the embedded templates filesystem
func Templates() fs.FS {
	templates, _ := fs.Sub(content, "templates")
	return templates
}

// Static returns the embedded static files filesystem
func Static() fs.FS {
	static, _ := fs.Sub(content, "static")
	return static
}
