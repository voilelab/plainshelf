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

func TestInsecureNetworkExposure(t *testing.T) {
	cases := []struct {
		name string
		mode SecurityMode
		addr string
		want bool
	}{
		{name: "none + non-loopback exposes", mode: SecurityModeNone, addr: "0.0.0.0:20000", want: true},
		{name: "none + all-interfaces exposes", mode: SecurityModeNone, addr: ":20000", want: true},
		{name: "none + loopback is local dev", mode: SecurityModeNone, addr: "127.0.0.1:20000", want: false},
		{name: "none + localhost is local dev", mode: SecurityModeNone, addr: "localhost:20000", want: false},
		{name: "local_token never exposes", mode: SecurityModeLocalToken, addr: "0.0.0.0:20000", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sec, err := NewSecurity(&SecurityConf{Mode: tc.mode})
			if err != nil {
				t.Fatalf("NewSecurity: %v", err)
			}
			if got := sec.InsecureNetworkExposure(tc.addr); got != tc.want {
				t.Fatalf("InsecureNetworkExposure(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}

	// A nil receiver stands in for the in-process embedders that build no
	// Security; it must never claim an exposure.
	if (*Security)(nil).InsecureNetworkExposure("0.0.0.0:20000") {
		t.Fatal("nil Security reported an exposure")
	}
}

// The log API carries the shelf's structure, access times and remote addresses,
// so it stays behind the token even where protect_read leaves the shelf itself
// readable without one. Only mode "none" -- an explicit "do not authenticate" --
// opens it.
func TestLogAPIRequiresTokenRegardlessOfProtectRead(t *testing.T) {
	localToken := func(protectRead bool) *SecurityConf {
		return &SecurityConf{
			Mode:                        SecurityModeLocalToken,
			ProtectRead:                 protectRead,
			AllowMissingOriginWithToken: new(true),
		}
	}

	cases := []struct {
		name string
		conf *SecurityConf
		path string
		want bool
	}{
		{"log list without protect_read", localToken(false), "/api/logs", true},
		{"log content without protect_read", localToken(false), "/api/logs/app-2024-01-02/content", true},
		{"log list with protect_read", localToken(true), "/api/logs", true},
		{"security disabled", &SecurityConf{Mode: SecurityModeNone}, "/api/logs", false},
		// The exception is scoped to the log routes: an ordinary read is still
		// governed by protect_read, and a path that merely starts with the same
		// letters is not a log route.
		{"book read without protect_read", localToken(false), "/api/shelves/default_shelf/books", false},
		{"unrelated path sharing the prefix", localToken(false), "/api/logsomething", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppWithSecurity(t, tc.conf)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			if got := app.security.requiresToken(req); got != tc.want {
				t.Fatalf("requiresToken(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// A rescan walks the shelf and writes nothing, so protect_read governs it the
// way it governs every other read. Until PSW-63 its method alone kept it behind
// the token, and the "refresh the book list" button failed with 401 under the
// shipped defaults -- local_token with protect_read off -- which the docs
// promise need no token.
func TestShelfScanFollowsProtectReadRatherThanItsMethod(t *testing.T) {
	const scanPath = "/api/shelves/default_shelf/scans"

	localToken := func(protectRead bool) *SecurityConf {
		return &SecurityConf{
			Mode:                        SecurityModeLocalToken,
			ProtectRead:                 protectRead,
			AllowMissingOriginWithToken: new(true),
		}
	}

	cases := []struct {
		name   string
		conf   *SecurityConf
		method string
		path   string
		want   bool
	}{
		{"rescan without protect_read", localToken(false), http.MethodPost, scanPath, false},
		{"rescan with protect_read", localToken(true), http.MethodPost, scanPath, true},
		{"security disabled", &SecurityConf{Mode: SecurityModeNone}, http.MethodPost, scanPath, false},
		// The exemption is scoped to the scan route and to POST: every other
		// mutation stays gated, and a path that merely ends the same way is not
		// a scan route.
		{"another mutation", localToken(false), http.MethodPost, "/api/shelves/default_shelf/trash", true},
		{"deleting the scan route", localToken(false), http.MethodDelete, scanPath, true},
		{"nested under the scan route", localToken(false), http.MethodPost, scanPath + "/abc", true},
		{"scans without a shelf", localToken(false), http.MethodPost, "/api/shelves//scans", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppWithSecurity(t, tc.conf)
			req := httptest.NewRequest(tc.method, tc.path, nil)

			if got := app.security.requiresToken(req); got != tc.want {
				t.Fatalf("requiresToken(%s %q) = %v, want %v", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

// Exempt from the token is not exempt from CSRF. The token gate is documented
// as a CSRF boundary, so the request that skips it still has to prove it is not
// a page acting on its own -- otherwise any site the household visits could
// make the server walk an SMB shelf.
func TestTokenExemptScanStillChecksOrigin(t *testing.T) {
	const scanPath = "/api/shelves/default_shelf/scans"

	cases := []struct {
		name    string
		origin  string
		referer string
		want    int
	}{
		// The Android client's native HTTP bridge sends neither header, and no
		// browser request can forge one it does not send.
		{name: "no origin", want: http.StatusOK},
		{name: "allowed origin", origin: "http://127.0.0.1:20000", want: http.StatusOK},
		{name: "cross-site origin", origin: "http://evil.example", want: http.StatusForbidden},
		{name: "cross-site referer", referer: "http://evil.example/page", want: http.StatusForbidden},
		{name: "unparseable origin", origin: "://", want: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppWithSecurity(t, &SecurityConf{
				Mode:                        SecurityModeLocalToken,
				AllowMissingOriginWithToken: new(true),
			})

			req := httptest.NewRequest(http.MethodPost, scanPath, nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}

			rec := httptest.NewRecorder()
			app.security.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// The log API is the reverse exception and is unchanged by the scan one: it
// needs a token whatever protect_read says, and a scan-shaped log path is still
// a log path.
func TestLogAPIStaysGatedAlongsideTheScanExemption(t *testing.T) {
	app := newTestAppWithSecurity(t, &SecurityConf{
		Mode:                        SecurityModeLocalToken,
		AllowMissingOriginWithToken: new(true),
	})

	for _, path := range []string{"/api/logs", "/api/logs/app-2024-01-02/scans"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		if !app.security.requiresToken(req) {
			t.Errorf("requiresToken(POST %q) = false, want true", path)
		}
	}
}
