package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
)

// Handle SPA fallback for all non-API GET requests
func (app *App) HandleSPAFallback(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(r.URL.Path, "/")
	if cleanPath == "" || !hasFileExtension(cleanPath) {
		// SPA fallback: serve index.html for root and all non-file paths
		data, err := fs.ReadFile(app.spaFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(app.injectSecurityBootstrap(data))
		return
	}

	app.spaHandler.ServeHTTP(w, r)
}

func (app *App) injectSecurityBootstrap(data []byte) []byte {
	if app.security == nil || !app.security.IsEnabled() || app.security.Token() == "" {
		return data
	}
	token, err := json.Marshal(app.security.Token())
	if err != nil {
		return data
	}
	header, err := json.Marshal(app.security.TokenHeader())
	if err != nil {
		return data
	}
	bootstrap := []byte(`<script>window.__PLAINSHELF_SECURITY__={token:` + string(token) + `,tokenHeader:` + string(header) + `};</script>`)
	marker := []byte("</head>")
	if idx := bytes.Index(data, marker); idx >= 0 {
		out := make([]byte, 0, len(data)+len(bootstrap))
		out = append(out, data[:idx]...)
		out = append(out, bootstrap...)
		out = append(out, data[idx:]...)
		return out
	}
	return append(bootstrap, data...)
}

func hasFileExtension(path string) bool {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return false
		}
		if path[i] == '.' {
			return true
		}
	}
	return false
}
