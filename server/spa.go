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

	// warnInsecurePublic makes the injected bootstrap carry an
	// insecurePublicAccess flag so the Web UI shows a persistent "no API auth"
	// warning. It is set only by the network server path when mode is none and
	// the listen address is not loopback; see App.SetInsecureNetworkWarning.
	warnInsecurePublic bool
}

// securityBootstrapPayload is the JSON shape written into
// window.__PLAINSHELF_SECURITY__. Fields are omitempty so mode none injects only
// the warning flag (it has no token) and a loopback none injects nothing at all.
type securityBootstrapPayload struct {
	Token                string `json:"token,omitempty"`
	TokenHeader          string `json:"tokenHeader,omitempty"`
	InsecurePublicAccess bool   `json:"insecurePublicAccess,omitempty"`
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
	payload := securityBootstrapPayload{}
	if h.security != nil && h.security.IsEnabled() && h.security.Token() != "" {
		payload.Token = h.security.Token()
		payload.TokenHeader = h.security.TokenHeader()
	}
	payload.InsecurePublicAccess = h.warnInsecurePublic
	if payload == (securityBootstrapPayload{}) {
		return data
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return data
	}
	bootstrap := []byte(`<script>window.__PLAINSHELF_SECURITY__=` + string(encoded) + `;</script>`)
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
