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
// window.__PLAINSHELF_SECURITY__. Fields are omitted when empty so mode none
// injects only the warning flag (it has no token) and a loopback none injects
// nothing at all.
type securityBootstrapPayload struct {
	Token                string `json:"token,omitempty"`
	TokenHeader          string `json:"tokenHeader,omitempty"`
	InsecurePublicAccess bool   `json:"insecurePublicAccess,omitzero"`
}

// fallback serves index.html for all non-API GET requests that do not name a
// file, so the SPA's own router can handle the path.
func (h *spaHandlers) fallback(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(r.URL.Path, "/")
	if cleanPath == "" || path.Ext(cleanPath) == "" {
		data, err := fs.ReadFile(h.fs, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// A fresh nonce per response: a reused or guessable one would let an
		// injected inline script claim it and defeat the policy.
		nonce := rand.Text()
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(nonce))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(h.injectSecurityBootstrap(data, nonce))
		return
	}

	h.files.ServeHTTP(w, r)
}

// contentSecurityPolicy is the browser-side backstop if the sanitizer is ever
// bypassed: script-src has no 'unsafe-inline' (the bootstrap carries the nonce)
// and framing is refused, so an XSS or clickjack cannot take the local token.
// Each relaxation serves something the app loads: img-src data: for the inlined
// bookcover.svg; img-src blob: for token-fetched illustrations
// (frontend/src/composables/useAssetSrc.ts); style-src 'unsafe-inline' for the
// author text colour frontend/src/utils/safeHtml.ts keeps through v-html (Vite
// emits no inline style or script); font-src data: for the Noto subsets Vite
// inlines, the rest loading from 'self' (verified on a real page load). The rest
// is denied (object-, media-, worker-src) or same-origin. HSTS and violation
// reporting are out of scope: the default deployment is http://127.0.0.1.
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
