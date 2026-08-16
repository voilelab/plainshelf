package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

// newSecurityTestApp builds an app under the given security configuration. What
// the security layer does to a request is pinned by the contract tests; this is
// for the parts that are reached past the HTTP surface.
func newSecurityTestApp(t *testing.T, conf *SecurityConf) *App {
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

	// Closing an app whose initial scan is still running is not what is under
	// test here, so the scan is allowed to finish first.
	waitForShelves(t, app)

	return app
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
			app := newSecurityTestApp(t, tc.conf)
			req := httptest.NewRequest(http.MethodGet, imagePath, nil)

			if got := app.handlers.core.cacheVisibility(req); got != tc.want {
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
