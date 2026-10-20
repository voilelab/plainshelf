package server

import (
	"strings"
	"testing"
)

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

func TestInjectSecurityBootstrapEscapesScriptClose(t *testing.T) {
	const page = "<html><head></head><body></body></html>"

	security, err := NewSecurity(&SecurityConf{
		Mode:        SecurityModeLocalToken,
		TokenHeader: `X-Evil</script><script>alert(1)</script>`,
	})
	if err != nil {
		t.Fatalf("NewSecurity(local_token): %v", err)
	}

	h := &spaHandlers{security: security}
	out := string(h.injectSecurityBootstrap([]byte(page), "test-nonce-value"))

	if strings.Contains(out, "</script><script>") {
		t.Fatalf("token header closed the bootstrap script: %q", out)
	}
	if !strings.Contains(out, `X-Evil\u003c/script`) {
		t.Fatalf("token header not escaped: %q", out)
	}
}
