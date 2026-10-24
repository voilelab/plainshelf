package crosscut_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
)

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

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"spa index", env.Get("/")},
		{"api read", env.Get(apitest.BooksURL())},
		{"api mutation", env.Post(apitest.SettingPath, strings.NewReader(mutationBody))},
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

func TestSecurityHeadersSecurityDisabled(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(&server.SecurityConf{Mode: server.SecurityModeNone}))

	assertStaticSecurityHeaders(t, env.Get("/"), "spa index (security none)")
	assertStaticSecurityHeaders(t, env.Get(apitest.BooksURL()), "api read (security none)")
}

var cspScriptSrcRE = regexp.MustCompile(`script-src ([^;]*)`)

func TestContentSecurityPolicyOnIndexContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))

	rec := env.Get("/")
	apitest.AssertStatus(t, rec, http.StatusOK)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("index response carries no Content-Security-Policy header")
	}

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

func TestBootstrapScriptCarriesCSPNonce(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))

	headerNonce := func() string {
		rec := env.Get("/")
		apitest.AssertStatus(t, rec, http.StatusOK)

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
