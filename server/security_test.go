package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newSecurityTestEnv(t *testing.T, conf *SecurityConf) *apiTestEnv {
	t.Helper()
	app, err := NewApp(&AppConf{
		ShelfPath:        t.TempDir(),
		StorePath:        t.TempDir(),
		CoverToJPG:       false,
		ReadHistoryLimit: 2,
		Security:         conf,
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("Close app: %v", err)
		}
	})
	return &apiTestEnv{app: app, handler: app.Handler()}
}

func TestSecurityLocalTokenProtectsMutatingAPI(t *testing.T) {
	env := newSecurityTestEnv(t, &SecurityConf{
		Mode:                        SecurityModeLocalToken,
		AllowMissingOriginWithToken: boolPtr(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	})

	if len(env.app.SecurityToken()) < 32 {
		t.Fatalf("security token length = %d, want at least 32", len(env.app.SecurityToken()))
	}

	rec := env.doRaw(httptest.NewRequest(http.MethodGet, "/health", nil))
	assertStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "1" {
		t.Fatalf("health body = %q, want 1", rec.Body.String())
	}

	rec = env.doRaw(httptest.NewRequest(http.MethodGet, "/api/books", nil))
	assertStatus(t, rec, http.StatusOK)

	rec = env.doRaw(httptest.NewRequest(http.MethodPost, "/api/read_history?book_id=book-1", nil))
	assertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, "/api/read_history?book_id=book-1", nil)
	req.Header.Set(env.app.SecurityTokenHeader(), "wrong-token")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusUnauthorized)

	req = httptest.NewRequest(http.MethodPost, "/api/read_history?book_id=book-1", nil)
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)
}

func TestSecurityOriginAndCORS(t *testing.T) {
	env := newSecurityTestEnv(t, &SecurityConf{
		Mode:                        SecurityModeLocalToken,
		AllowMissingOriginWithToken: boolPtr(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/read_history?book_id=book-1", nil)
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	req.Header.Set("Origin", "http://evil.example")
	rec := env.doRaw(req)
	assertStatus(t, rec, http.StatusForbidden)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed CORS origin header = %q, want empty", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/read_history?book_id=book-1", nil)
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	req.Header.Set("Origin", "http://localhost:20000")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("allowed CORS origin header = %q, want http://localhost:20000", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/read_history?book_id=book-2", nil)
	req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
	req.Header.Set("Referer", "http://localhost:20000/books")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodOptions, "/api/read_history", nil)
	req.Header.Set("Origin", "http://localhost:20000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20000" {
		t.Fatalf("preflight origin header = %q, want http://localhost:20000", got)
	}
}

func TestSecurityProtectReadOption(t *testing.T) {
	env := newSecurityTestEnv(t, &SecurityConf{
		Mode:                        SecurityModeLocalToken,
		ProtectRead:                 true,
		AllowMissingOriginWithToken: boolPtr(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	})

	rec := env.doRaw(httptest.NewRequest(http.MethodGet, "/api/books", nil))
	assertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, "/api/books", nil)
	req.Header.Set("Authorization", "Bearer "+env.app.SecurityToken())
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusOK)
}

func TestValidateSecurityForListenAddr(t *testing.T) {
	if err := ValidateSecurityForListenAddr(nil, "127.0.0.1:20000"); err != nil {
		t.Fatalf("loopback validation returned error: %v", err)
	}
	if err := ValidateSecurityForListenAddr(nil, "0.0.0.0:20000"); err == nil {
		t.Fatal("non-loopback validation without explicit mode succeeded, want error")
	}
	if err := ValidateSecurityForListenAddr(&SecurityConf{Mode: SecurityModeNone}, "0.0.0.0:20000"); err != nil {
		t.Fatalf("explicit mode validation returned error: %v", err)
	}
}
