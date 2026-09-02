package server

import (
	"strings"
	"testing"
)

// The bootstrap injected into index.html is the only channel the ticket allows
// the Web UI's "no API auth" warning to travel through (no new API endpoint), so
// these cases pin exactly which flags each security posture emits.
func TestInjectSecurityBootstrap(t *testing.T) {
	const page = "<html><head></head><body></body></html>"

	localToken, err := NewSecurity(&SecurityConf{Mode: SecurityModeLocalToken})
	if err != nil {
		t.Fatalf("NewSecurity(local_token): %v", err)
	}
	none, err := NewSecurity(&SecurityConf{Mode: SecurityModeNone})
	if err != nil {
		t.Fatalf("NewSecurity(none): %v", err)
	}

	cases := []struct {
		name         string
		security     *Security
		warn         bool
		wantInjected bool
		wantToken    bool
		wantInsecure bool
	}{
		{
			name:         "local_token injects token, no warning",
			security:     localToken,
			warn:         false,
			wantInjected: true,
			wantToken:    true,
			wantInsecure: false,
		},
		{
			name:         "none + loopback injects nothing",
			security:     none,
			warn:         false,
			wantInjected: false,
		},
		{
			name:         "none + non-loopback injects only the warning",
			security:     none,
			warn:         true,
			wantInjected: true,
			wantToken:    false,
			wantInsecure: true,
		},
	}

	// The CSP served with the same response admits this inline bootstrap only by
	// nonce, so the injected <script> must carry it verbatim.
	const nonce = "test-nonce-value"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &spaHandlers{security: tc.security, warnInsecurePublic: tc.warn}
			out := string(h.injectSecurityBootstrap([]byte(page), nonce))

			injected := strings.Contains(out, "window.__PLAINSHELF_SECURITY__")
			if injected != tc.wantInjected {
				t.Fatalf("injected = %v, want %v (out=%q)", injected, tc.wantInjected, out)
			}
			if !tc.wantInjected {
				if out != page {
					t.Fatalf("expected page unchanged, got %q", out)
				}
				return
			}
			// Injected before </head> so it runs before the SPA bundle reads it.
			if strings.Index(out, "window.__PLAINSHELF_SECURITY__") > strings.Index(out, "</head>") {
				t.Fatalf("bootstrap injected after </head>: %q", out)
			}
			if !strings.Contains(out, `<script nonce="`+nonce+`">`) {
				t.Fatalf("bootstrap script missing CSP nonce %q: %q", nonce, out)
			}
			if got := strings.Contains(out, `"token"`); got != tc.wantToken {
				t.Fatalf("token present = %v, want %v (out=%q)", got, tc.wantToken, out)
			}
			if got := strings.Contains(out, `"insecurePublicAccess":true`); got != tc.wantInsecure {
				t.Fatalf("insecurePublicAccess present = %v, want %v (out=%q)", got, tc.wantInsecure, out)
			}
		})
	}
}
