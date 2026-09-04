package contract_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// The browser-hardening headers every response must carry, whatever the route
// and whatever the security mode. They are restated here rather than imported:
// the exact header names and values are part of the contract this ticket adds,
// so a change to either has to be a change to this test too.
var staticSecurityHeaders = map[string]string{
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Referrer-Policy":        "no-referrer",
}

func assertStaticSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder, where string) {
	t.Helper()
	for name, want := range staticSecurityHeaders {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s: header %s = %q, want %q", where, name, got, want)
		}
	}
}

// TestSecurityHeadersOnEveryResponseContract pins that the static hardening
// headers ride on the SPA document, an API read, an API mutation, the health
// check, and an unknown-API 404 alike -- the point of the header is that no
// response is exempt.
func TestSecurityHeadersOnEveryResponseContract(t *testing.T) {
	env := New(t, WithSecurity(LocalTokenSecurity()))

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"spa index", env.Get("/")},
		{"api read", env.Get(BooksURL())},
		{"api mutation", env.Post(SettingPath, strings.NewReader(mutationBody))},
		{"health", env.Get("/health")},
		{"unknown api", env.Get("/api/does-not-exist")},
		{"static asset 404", env.Get("/assets/missing.js")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStaticSecurityHeaders(t, tc.rec, tc.name)
		})
	}
}

// TestSecurityHeadersWithSecurityDisabledContract pins that the hardening
// headers do not depend on the token gate: they are just as present when the
// API security mode is none, because they defend the browser, not the API.
func TestSecurityHeadersWithSecurityDisabledContract(t *testing.T) {
	env := New(t, WithSecurity(&server.SecurityConf{Mode: server.SecurityModeNone}))

	assertStaticSecurityHeaders(t, env.Get("/"), "spa index (security none)")
	assertStaticSecurityHeaders(t, env.Get(BooksURL()), "api read (security none)")
}

var cspScriptSrcRE = regexp.MustCompile(`script-src ([^;]*)`)

// TestContentSecurityPolicyOnIndexContract pins the document CSP the ticket
// requires: present on the SPA index, carrying the minimum directive set, and
// keeping script-src free of 'unsafe-inline' by nonce-ing the one inline
// bootstrap script instead.
func TestContentSecurityPolicyOnIndexContract(t *testing.T) {
	env := New(t, WithSecurity(LocalTokenSecurity()))

	rec := env.Get("/")
	AssertStatus(t, rec, http.StatusOK)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("index response carries no Content-Security-Policy header")
	}

	// The directives the ticket names as the required floor.
	for _, directive := range []string{
		"default-src 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing required directive %q\nfull policy: %s", directive, csp)
		}
	}

	scriptSrc := cspScriptSrcRE.FindStringSubmatch(csp)
	if scriptSrc == nil {
		t.Fatalf("CSP has no script-src directive: %s", csp)
	}
	if strings.Contains(scriptSrc[1], "'unsafe-inline'") {
		t.Errorf("script-src must not allow 'unsafe-inline', got %q", scriptSrc[1])
	}
	if !strings.Contains(scriptSrc[1], "'nonce-") {
		t.Errorf("script-src must carry a nonce for the bootstrap script, got %q", scriptSrc[1])
	}
}

var (
	cspNonceRE       = regexp.MustCompile(`'nonce-([^']+)'`)
	bootstrapNonceRE = regexp.MustCompile(`<script nonce="([^"]+)">window\.__PLAINSHELF_SECURITY__`)
)

// TestBootstrapScriptCarriesCSPNonceContract pins that the nonce announced in
// the CSP header is the nonce written on the injected bootstrap <script>, so
// the one inline script the app needs is exactly the one the policy admits.
// A fresh request must mint a fresh nonce.
func TestBootstrapScriptCarriesCSPNonceContract(t *testing.T) {
	env := New(t, WithSecurity(LocalTokenSecurity()))

	headerNonce := func() string {
		rec := env.Get("/")
		AssertStatus(t, rec, http.StatusOK)

		header := cspNonceRE.FindStringSubmatch(rec.Header().Get("Content-Security-Policy"))
		if header == nil {
			t.Fatalf("no nonce in CSP header: %s", rec.Header().Get("Content-Security-Policy"))
		}
		script := bootstrapNonceRE.FindStringSubmatch(rec.Body.String())
		if script == nil {
			t.Fatalf("bootstrap script has no nonce attribute:\n%s", rec.Body.String())
		}
		if header[1] != script[1] {
			t.Fatalf("CSP nonce %q does not match bootstrap script nonce %q", header[1], script[1])
		}
		return header[1]
	}

	if first, second := headerNonce(), headerNonce(); first == second {
		t.Fatalf("nonce %q was reused across two responses; it must be per-response", first)
	}
}
