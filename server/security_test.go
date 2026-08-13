package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

// Representative mutation used throughout this file. Updating this setting
// needs no imported book and answers 204 on success.
const mutationPath = "/api/setting/cover_to_jpg"
const mutationBody = `true`

func newSecurityTestEnv(t *testing.T, conf *SecurityConf) *apiTestEnv {
	t.Helper()
	app, err := NewApp(&AppConf{
		Shelves: []*shelf.ShelfConfWithID{
			{
				ID: "default_shelf",
				ShelfConf: shelf.ShelfConf{
					LibRoot: t.TempDir(),
				},
			},
		},
		StorePath:  t.TempDir(),
		CoverToJPG: false,
		Security:   conf,
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
		AllowMissingOriginWithToken: new(true),
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

	rec = env.doRaw(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
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

func TestSecurityOriginAndCORS(t *testing.T) {
	env := newSecurityTestEnv(t, &SecurityConf{
		Mode:                        SecurityModeLocalToken,
		AllowMissingOriginWithToken: new(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	})

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

func TestSecurityProtectReadOption(t *testing.T) {
	env := newSecurityTestEnv(t, &SecurityConf{
		Mode:                        SecurityModeLocalToken,
		ProtectRead:                 true,
		AllowMissingOriginWithToken: new(true),
		AllowedOrigins:              []string{"http://localhost:20000"},
	})

	rec := env.doRaw(httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil))
	assertStatus(t, rec, http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodGet, "/api/shelves/default_shelf/books", nil)
	req.Header.Set("Authorization", "Bearer "+env.app.SecurityToken())
	rec = env.doRaw(req)
	assertStatus(t, rec, http.StatusOK)
}

// A response the token gate protected must not be storable by a shared cache:
// the token rides in a header a cache does not key on, so a stored copy could
// answer a later request that never reached the gate.
func TestCacheVisibilityFollowsTheTokenGate(t *testing.T) {
	const imagePath = "/api/shelves/default_shelf/books/some-book/cover"

	cases := []struct {
		name string
		conf *SecurityConf
		want string
	}{
		{
			name: "reads unprotected",
			conf: &SecurityConf{
				Mode:                        SecurityModeLocalToken,
				AllowMissingOriginWithToken: new(true),
			},
			want: "public",
		},
		{
			name: "reads protected",
			conf: &SecurityConf{
				Mode:                        SecurityModeLocalToken,
				ProtectRead:                 true,
				AllowMissingOriginWithToken: new(true),
			},
			want: "private",
		},
		{
			name: "security disabled",
			conf: &SecurityConf{Mode: SecurityModeNone},
			want: "public",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newSecurityTestEnv(t, tc.conf)
			req := httptest.NewRequest(http.MethodGet, imagePath, nil)

			if got := env.app.cacheVisibility(req); got != tc.want {
				t.Fatalf("cacheVisibility = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateSecurityForListenAddr(t *testing.T) {
	if err := ValidateSecurityForListenAddr(nil, "127.0.0.1:20000"); err != nil {
		t.Fatalf("loopback validation returned error: %v", err)
	}
	if err := ValidateSecurityForListenAddr(nil, "0.0.0.0:20000"); err == nil {
		t.Fatal("non-loopback validation without explicit mode succeeded, want error")
	}
	// Empty host (e.g. ":20000") binds all interfaces — must require explicit mode.
	if err := ValidateSecurityForListenAddr(nil, ":20000"); err == nil {
		t.Fatal("all-interfaces addr validation without explicit mode succeeded, want error")
	}
	if err := ValidateSecurityForListenAddr(&SecurityConf{Mode: SecurityModeNone}, "0.0.0.0:20000"); err != nil {
		t.Fatalf("explicit mode validation returned error: %v", err)
	}
}
