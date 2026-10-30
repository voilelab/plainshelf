package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json/v2"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
)

type spaHandlers struct {
	fs       fs.FS
	files    http.Handler
	security *Security

	warnInsecurePublic bool
}

type securityBootstrapPayload struct {
	Token                string `json:"token,omitempty"`
	TokenHeader          string `json:"tokenHeader,omitempty"`
	InsecurePublicAccess bool   `json:"insecurePublicAccess,omitzero"`
}

func (h *spaHandlers) fallback(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(r.URL.Path, "/")
	if cleanPath == "" || path.Ext(cleanPath) == "" {
		data, err := fs.ReadFile(h.fs, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}

		nonce := rand.Text()
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(nonce))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(h.injectSecurityBootstrap(data, nonce))
		return
	}

	h.files.ServeHTTP(w, r)
}

func contentSecurityPolicy(nonce string) string {
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'nonce-" + nonce + "'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"manifest-src 'self'",
		"object-src 'none'",
		"media-src 'none'",
		"worker-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

func (h *spaHandlers) injectSecurityBootstrap(data []byte, nonce string) []byte {
	payload := securityBootstrapPayload{}
	if h.security != nil && h.security.IsEnabled() && h.security.Token() != "" {
		payload.Token = h.security.Token()
		payload.TokenHeader = h.security.TokenHeader()
	}
	payload.InsecurePublicAccess = h.warnInsecurePublic
	if payload == (securityBootstrapPayload{}) {
		return data
	}
	encoded, err := json.Marshal(payload, jsonopt.API())
	if err != nil {
		return data
	}
	// token_header comes from the config file and reaches this inline script
	// verbatim. "<" is the only character that could close the element, and a
	// \u003c escape survives JSON.parse unchanged; v2 does not escape it, so
	// this replacement is the only thing standing there.
	encoded = bytes.ReplaceAll(encoded, []byte("<"), []byte(`\u003c`))
	bootstrap := []byte(`<script nonce="` + nonce + `">window.__PLAINSHELF_SECURITY__=` + string(encoded) + `;</script>`)
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
