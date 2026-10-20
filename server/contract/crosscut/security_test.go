package crosscut_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"gopkg.in/yaml.v3"
)

const mutationBody = `true`

func TestAPISecurityLocalTokenProtectsMutatingAPIContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))

	if len(env.App.SecurityToken()) < 32 {
		t.Fatalf("security token length = %d, want at least 32", len(env.App.SecurityToken()))
	}

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, "/health", nil))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "1" {
		t.Fatalf("health body = %q, want 1", rec.Body.String())
	}

	rec = env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.BooksURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusOK)

	rec = env.DoRaw(httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody)))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), "wrong-token")
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	req = httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusNoContent)
}

func TestAPISecurityOriginAndCORSContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))

	req := httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	req.Header.Set("Origin", "http://evil.example")
	rec := env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusForbidden)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed CORS origin header = %q, want empty", got)
	}

	req = httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	req.Header.Set("Origin", "http://localhost:20000")
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("allowed CORS origin header = %q, want http://localhost:20000", got)
	}

	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-Id" {
		t.Fatalf("exposed headers = %q, want X-Request-Id", got)
	}

	req = httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	req.Header.Set("Referer", "http://localhost:20000/books")
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodOptions, apitest.SettingPath, nil)
	req.Header.Set("Origin", "http://localhost:20000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("preflight origin header = %q, want http://localhost:20000", got)
	}
}

// TestAPISecurityContainerDefaultConfigContract mounts the shipped container
// default on the real router: the image runs docker/config.yaml unmodified, so
// its parsed security block is what a user who forgets a port mapping is
// protected by, and the block is asserted here as configured rather than as a
// synthetic one the test wrote itself.
//
// What that block has to say for itself — mode local_token, and a listen-addr
// guard that passes — needs no request and is pinned in
// server/conf_container_test.go.
func TestAPISecurityContainerDefaultConfigContract(t *testing.T) {
	confPath := filepath.Join(apitest.RepoRoot(t), "docker", "config.yaml")
	bs, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read %s: %v", confPath, err)
	}

	var conf server.SrvConf
	if err := yaml.Unmarshal(bs, &conf); err != nil {
		t.Fatalf("parse %s: %v", confPath, err)
	}
	if conf.AppConf == nil || conf.AppConf.Security == nil {
		t.Fatalf("container config has no app_conf.security block")
	}

	env := apitest.New(t, apitest.WithSecurity(conf.AppConf.Security))

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody)))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusNoContent)
}

func TestAPISecurityProtectReadOptionContract(t *testing.T) {
	security := apitest.LocalTokenSecurity()
	security.ProtectRead = true
	env := apitest.New(t, apitest.WithSecurity(security))

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, apitest.BooksURL(), nil))
	apitest.AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, apitest.BooksURL(), nil)
	req.Header.Set("Authorization", "Bearer "+env.App.SecurityToken())
	rec = env.DoRaw(req)
	apitest.AssertStatus(t, rec, http.StatusOK)
}

// A browser leaves the default port out of Origin and Referer, so the same
// origin written with or without it has to match; nothing else may merge.
func TestAPISecurityOriginDefaultPortContract(t *testing.T) {
	cases := []struct {
		name    string
		allowed string
		origin  string
		ok      bool
	}{
		{"http listed with :80", "http://demo.example:80", "http://demo.example", true},
		{"https listed with :443", "https://demo.example:443", "https://demo.example", true},
		{"http request with :80", "http://demo.example", "http://demo.example:80", true},
		{"https request with :443", "https://demo.example", "https://demo.example:443", true},
		{"non-default port", "http://demo.example", "http://demo.example:8080", false},
		{"non-default port listed", "http://demo.example:8080", "http://demo.example", false},
		{"https on :80", "https://demo.example:80", "https://demo.example", false},
		{"http on :443", "http://demo.example:443", "http://demo.example", false},
		{"unlisted host", "http://demo.example:80", "http://other.example", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			security := apitest.LocalTokenSecurity()
			security.AllowedOrigins = []string{tc.allowed}
			env := apitest.New(t, apitest.WithSecurity(security))
			want := http.StatusForbidden
			if tc.ok {
				want = http.StatusNoContent
			}

			req := httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
			req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
			req.Header.Set("Origin", tc.origin)
			rec := env.DoRaw(req)
			apitest.AssertStatus(t, rec, want)
			wantACAO := ""
			if tc.ok {
				wantACAO = tc.origin
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != wantACAO {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, wantACAO)
			}

			req = httptest.NewRequest(http.MethodPost, apitest.SettingPath, strings.NewReader(mutationBody))
			req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
			req.Header.Set("Referer", tc.origin+"/books")
			rec = env.DoRaw(req)
			apitest.AssertStatus(t, rec, want)
		})
	}
}
