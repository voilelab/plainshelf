package frontend

import (
	"embed"
	"io/fs"
)

// all: is required so files whose names start with "_" (e.g. rolldown's
// shared _plugin-vue_export-helper-*.js chunk) are embedded too; the
// default pattern silently skips them and the SPA 404s at runtime.
//
//go:embed all:dist
var distFS embed.FS

var WebFS fs.FS

func init() {
	var err error
	WebFS, err = fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
}
