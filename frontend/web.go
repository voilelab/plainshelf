//go:build !(js && wasm) && !android

package frontend

import (
	"embed"
	"io/fs"
)

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
