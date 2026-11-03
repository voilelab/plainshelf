package wailsshell

import (
	"regexp"
	"slices"
	"strings"
)

const runtimeScripts = `<script src="/wails/ipc.js"></script><script src="/wails/runtime.js"></script>`

var headOpenTag = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)

// Wails v2.16.0 injects ipc.js and runtime.js only into HTML served for a path
// ending in "/" or "/index.html" (pkg/assetserver isRuntimeInjectionMatch), so
// an SPA route reloaded in place would load without window.go.
func WithRuntime(requestPath string, page []byte) []byte {
	if requestPath == "" || strings.HasSuffix(requestPath, "/") || strings.HasSuffix(requestPath, "/index.html") {
		return page
	}
	head := headOpenTag.FindIndex(page)
	if head == nil {
		return page
	}
	return slices.Concat(page[:head[1]], []byte(runtimeScripts), page[head[1]:])
}
