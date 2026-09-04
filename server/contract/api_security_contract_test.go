package contract_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"gopkg.in/yaml.v3"
)

// Representative route used throughout this file. This setting needs no
// imported book: reading it answers 200 and updating it 204, so one path
// exercises both sides of the token gate.
const SettingPath = "/api/setting/cover_to_jpg"
const mutationBody = `true`

// LocalTokenSecurity is the configuration these tests share: the local-token
// mode a desktop client runs under, with one allowed browser origin.
func LocalTokenSecurity() *server.SecurityConf {
	return &server.SecurityConf{
		Mode:                        server.SecurityModeLocalToken,
		AllowMissingOriginWithToken: new(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	}
}

func TestAPISecurityLocalTokenProtectsMutatingAPIContract(t *testing.T) {
	env := New(t, WithSecurity(LocalTokenSecurity()))

	if len(env.App.SecurityToken()) < 32 {
		t.Fatalf("security token length = %d, want at least 32", len(env.App.SecurityToken()))
	}

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, "/health", nil))
	AssertStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "1" {
		t.Fatalf("health body = %q, want 1", rec.Body.String())
	}

	rec = env.DoRaw(httptest.NewRequest(http.MethodGet, BooksURL(), nil))
	AssertStatus(t, rec, http.StatusOK)

	rec = env.DoRaw(httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody)))
	AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), "wrong-token")
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusUnauthorized)

	req = httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusNoContent)
}

func TestAPISecurityOriginAndCORSContract(t *testing.T) {
	env := New(t, WithSecurity(LocalTokenSecurity()))

	req := httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	req.Header.Set("Origin", "http://evil.example")
	rec := env.DoRaw(req)
	AssertStatus(t, rec, http.StatusForbidden)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed CORS origin header = %q, want empty", got)
	}

	req = httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	req.Header.Set("Origin", "http://localhost:20000")
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("allowed CORS origin header = %q, want http://localhost:20000", got)
	}
	// The request ID is what a user quotes in a bug report, so a browser on an
	// allowed origin has to be able to read it rather than only send it.
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-Id" {
		t.Fatalf("exposed headers = %q, want X-Request-Id", got)
	}

	req = httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	req.Header.Set("Referer", "http://localhost:20000/books")
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodOptions, SettingPath, nil)
	req.Header.Set("Origin", "http://localhost:20000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("preflight origin header = %q, want http://localhost:20000", got)
	}
}

// TestAPISecurityContainerDefaultConfigContract pins the shipped container
// default. The image runs docker/config.yaml unmodified, so its parsed
// security block is what a user who forgets a port mapping is protected by. A
// regression to mode: "none" would silently answer anonymous writes on any
// exposed port, which is exactly what this default exists to prevent.
func TestAPISecurityContainerDefaultConfigContract(t *testing.T) {
	confPath := filepath.Join("..", "..", "docker", "config.yaml")
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
	if got := conf.AppConf.Security.Mode; got != server.SecurityModeLocalToken {
		t.Fatalf("container config security mode = %q, want %q", got, server.SecurityModeLocalToken)
	}

	// addr stays 0.0.0.0:20000 (required inside the container) and the listen-addr
	// guard still passes because the mode is set explicitly.
	if conf.ServerConf == nil {
		t.Fatalf("container config has no server_conf block")
	}
	if err := server.ValidateSecurityForListenAddr(conf.AppConf.Security, conf.ServerConf.Addr); err != nil {
		t.Fatalf("ValidateSecurityForListenAddr(%q) = %v, want nil", conf.ServerConf.Addr, err)
	}

	// The parsed default protects mutating requests: no token -> 401, token -> 204.
	env := New(t, WithSecurity(conf.AppConf.Security))

	rec := env.DoRaw(httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody)))
	AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, SettingPath, strings.NewReader(mutationBody))
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusNoContent)
}

func TestAPISecurityProtectReadOptionContract(t *testing.T) {
	security := LocalTokenSecurity()
	security.ProtectRead = true
	env := New(t, WithSecurity(security))

	rec := env.DoRaw(httptest.NewRequest(http.MethodGet, BooksURL(), nil))
	AssertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, BooksURL(), nil)
	req.Header.Set("Authorization", "Bearer "+env.App.SecurityToken())
	rec = env.DoRaw(req)
	AssertStatus(t, rec, http.StatusOK)
}
