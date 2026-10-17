//go:build (js && wasm) || android

package frontend

import (
	"embed"
	"io/fs"
)

// WebFS is empty in the browser and Android builds: the page, or the APK's
// assets, serve the frontend itself.
var WebFS fs.FS = embed.FS{}
