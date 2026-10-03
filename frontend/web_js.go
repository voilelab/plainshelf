//go:build js && wasm

package frontend

import (
	"embed"
	"io/fs"
)

// WebFS is empty in the browser build: the page serves the frontend itself.
var WebFS fs.FS = embed.FS{}
