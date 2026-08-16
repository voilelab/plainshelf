package contract_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// Representative mutation used throughout this file. Updating this setting
// needs no imported book and answers 204 on success.
const mutationPath = "/api/setting/cover_to_jpg"
const mutationBody = `true`

// localTokenSecurity is the configuration these tests share: the local-token
// mode a desktop client runs under, with one allowed browser origin.
func localTokenSecurity() *server.SecurityConf {
	return &server.SecurityConf{
		Mode:                        server.SecurityModeLocalToken,
		AllowMissingOriginWithToken: new(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	}
}

func TestAPISecurityLocalTokenProtectsMutatingAPIContract(t *testing.T) {
	env := newAPITestEnv(t, withSecurity(localTokenSecurity()))

	if len(env.app.SecurityToken()) < 32 {
		t.Fatalf("security token length = %d, want at least 32", len(env.app.SecurityToken()))
	}

	rec := env.doRaw(httptest.NewRequest(http.MethodGet, "/health", nil))
	assertStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "1" {
		t.Fatalf("health body = %q, want 1", rec.Body.String())
	}

	rec = env.doRaw(httptest.NewRequest(http.MethodGet, booksURL(), nil))
	assertStatus(t, rec, http.StatusOK)

	rec = env.doRaw(httptest.NewRequest(http.MethodPost, mutationPath, strings.NewReader(mutationBody)))
	assertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, mutationPath, strings.NewReader(mutationBody))
	req.Header.Set(env.app.SecurityTokenHeader(), "wrong-token")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusUnauthorized)

	req = httptest.NewRequest(http.MethodPost, mutationPath, strings.NewReader(mutationBody))
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)
}

func TestAPISecurityOriginAndCORSContract(t *testing.T) {
	env := newAPITestEnv(t, withSecurity(localTokenSecurity()))

	req := httptest.NewRequest(http.MethodPost, mutationPath, strings.NewReader(mutationBody))
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	req.Header.Set("Origin", "http://evil.example")
	rec := env.doRaw(req)
	assertStatus(t, rec, http.StatusForbidden)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed CORS origin header = %q, want empty", got)
	}

	req = httptest.NewRequest(http.MethodPost, mutationPath, strings.NewReader(mutationBody))
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	req.Header.Set("Origin", "http://localhost:20000")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("allowed CORS origin header = %q, want http://localhost:20000", got)
	}

	req = httptest.NewRequest(http.MethodPost, mutationPath, strings.NewReader(mutationBody))
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	req.Header.Set("Referer", "http://localhost:20000/books")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodOptions, mutationPath, nil)
	req.Header.Set("Origin", "http://localhost:20000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("preflight origin header = %q, want http://localhost:20000", got)
	}
}

func TestAPISecurityProtectReadOptionContract(t *testing.T) {
	security := localTokenSecurity()
	security.ProtectRead = true
	env := newAPITestEnv(t, withSecurity(security))

	rec := env.doRaw(httptest.NewRequest(http.MethodGet, booksURL(), nil))
	assertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, booksURL(), nil)
	req.Header.Set("Authorization", "Bearer "+env.app.SecurityToken())
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusOK)
}
