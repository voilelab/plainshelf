package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			app := newTestAppWithSecurity(t, tc.conf)
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
