package server

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
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
	if cleanPath == "" || !hasFileExtension(cleanPath) {
		data, err := fs.ReadFile(h.fs, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		nonce, err := generateCSPNonce()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(nonce))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(h.injectSecurityBootstrap(data, nonce))
		return
	}

	h.files.ServeHTTP(w, r)
}

// generateCSPNonce returns a fresh base64 nonce for one HTML response's CSP.
// It must be unpredictable and unique per response: a reused or guessable nonce
// would let an injected inline script claim it and defeat the policy.
func generateCSPNonce() (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(nonceBytes), nil
}

// contentSecurityPolicy is the document policy sent with index.html. It is the
// browser-side backstop the ticket asks for: if the sanitizer is ever bypassed,
// the policy still refuses to run injected script (no 'unsafe-inline' in
// script-src — the one inline script, the bootstrap below, carries the nonce)
// and refuses to be framed, so a single XSS or clickjack cannot also carry off
// the local token embedded in this page.
//
// The relaxations are deliberate and each is forced by something the app really
// loads:
//   - img-src data: — the default cover (bookcover.svg) is small enough that the
//     build inlines it as a data: URI.
//   - img-src blob: — reader illustration assets are fetched with the token and
//     shown from object URLs (frontend/src/composables/useAssetSrc.ts).
//   - style-src 'unsafe-inline' — sanitized book text keeps an inline
//     `style="color: …"` attribute (frontend/src/utils/safeHtml.ts), rendered
//     through v-html; without it the reader loses author text colour. Vite emits
//     no inline <style> and no inline <script>, so nothing else needs it.
//   - font-src data: — the build inlines the Noto Sans TC subsets small enough
//     to fall under Vite's asset-inline limit as data: URIs; the rest load from
//     'self'. Verified against a real page load, not assumed.
//
// Everything the app never uses is denied outright (object-, media-, worker-src)
// or held to same-origin. HSTS and violation reporting are out of scope: the
// default deployment is http://127.0.0.1 and local-first.
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
	encoded, err := json.Marshal(payload)
	if err != nil {
		return data
	}
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
