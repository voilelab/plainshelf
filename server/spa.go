package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
)

// spaHandlers serves the embedded frontend. It holds security because the
// index it serves carries the token the frontend bootstraps from.
type spaHandlers struct {
	fs       fs.FS
	files    http.Handler
	security *Security
}

// fallback serves index.html for all non-API GET requests that do not name a
// file, so the SPA's own router can handle the path.
func (h *spaHandlers) fallback(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(r.URL.Path, "/")
	if cleanPath == "" || !hasFileExtension(cleanPath) {
		data, err := fs.ReadFile(h.fs, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(h.injectSecurityBootstrap(data))
		return
	}

	h.files.ServeHTTP(w, r)
}

func (h *spaHandlers) injectSecurityBootstrap(data []byte) []byte {
	if h.security == nil || !h.security.IsEnabled() || h.security.Token() == "" {
		return data
	}
	token, err := json.Marshal(h.security.Token())
	if err != nil {
		return data
	}
	header, err := json.Marshal(h.security.TokenHeader())
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
