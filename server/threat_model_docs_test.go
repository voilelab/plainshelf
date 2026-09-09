package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// docsThreatModel is the page that tells a reader what each deployment defends
// against. PSW-123 found three of its claims stronger than security.go actually
// delivers, which is worse than not documenting them at all: someone decides
// whether to put PlainShelf on their LAN by reading it. The tests below pin the
// two claims a test can hold — that the Tier B example works when copied, and
// that the page does not sell the desktop and reader apps as token-protected.
const docsThreatModel = "../docs/deployment-and-threat-model.md"

// TestDocsThreatModelTierBExampleAcceptsItsOwnOrigins runs the page's Tier B
// configuration through the real gate and writes from each origin it lists.
//
// allowed_origins replaces the loopback defaults rather than extending them
// (normalizeSecurityConf applies them only while the list is empty), so an
// example that names only LAN origins costs a reader every write made in a
// browser on the NAS itself — a 403 the page did not warn about. Copying the
// example has to be enough.
func TestDocsThreatModelTierBExampleAcceptsItsOwnOrigins(t *testing.T) {
	conf := tierBExampleConf(t)

	if err := ValidateSecurityForListenAddr(conf.AppConf.Security, conf.ServerConf.Addr); err != nil {
		t.Fatalf("documented Tier B example fails the listen-address security check: %v", err)
	}
	sec, err := NewSecurity(conf.AppConf.Security)
	if err != nil {
		t.Fatalf("documented Tier B example is rejected: %v", err)
	}

	// The two the example used to lose. They are what a browser on the NAS
	// itself sends, and the page tells the reader to set protect_read there.
	for _, origin := range []string{"http://127.0.0.1:20000", "http://localhost:20000"} {
		if !containsOrigin(conf.AppConf.Security.AllowedOrigins, origin) {
			t.Errorf("documented Tier B allowed_origins omits %s, so a browser on the "+
				"server itself is refused; the loopback defaults do not survive setting the list", origin)
		}
	}

	for _, origin := range conf.AppConf.Security.AllowedOrigins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/shelves/default_shelf/books", nil)
			req.Header.Set("Origin", origin)
			req.Header.Set(sec.TokenHeader(), sec.Token())

			rec := httptest.NewRecorder()
			sec.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("a write from %s documented as allowed got %d, want %d",
					origin, rec.Code, http.StatusOK)
			}
		})
	}
}

// TestDocsThreatModelDoesNotCallTheDesktopAppsTierA keeps the page from
// restoring the claim that the desktop and standalone reader are "effectively
// Tier A". The desktop hard-codes SecurityModeNone (desktop/app.go) and the
// reader does not reach this package at all, so neither runs the token gate
// Tier A is defined by; both open no network port, which is the true half the
// old sentence was built on.
func TestDocsThreatModelDoesNotCallTheDesktopAppsTierA(t *testing.T) {
	section := docsThreatModelSection(t, "### The desktop and standalone reader apps")

	if !strings.Contains(section, "`security.mode: none`") {
		t.Error("the desktop/reader section no longer names the mode desktop/app.go actually sets " +
			"(`security.mode: none`); a reader cannot check the claim against the code without it")
	}
	if strings.Contains(section, "Tier A") && !strings.Contains(section, "weaker than") {
		t.Error("the desktop/reader section mentions Tier A without saying its protection is weaker; " +
			"neither app runs the token gate that defines Tier A")
	}
}

// tierBExampleConf decodes the YAML block under the page's Tier B heading.
func tierBExampleConf(t *testing.T) SrvConf {
	t.Helper()

	const heading = "\n### Tier B — home LAN / NAS, including VPN remote\n"
	page := docsThreatModelPage(t)
	_, after, found := strings.Cut(page, heading)
	if !found {
		t.Fatalf("%s no longer has a %q section", docsThreatModel, strings.TrimSpace(heading))
	}
	_, after, found = strings.Cut(after, "```yaml\n")
	if !found {
		t.Fatalf("%s has no YAML block under Tier B", docsThreatModel)
	}
	block, _, found := strings.Cut(after, "```")
	if !found {
		t.Fatalf("%s has an unterminated YAML block under Tier B", docsThreatModel)
	}

	var conf SrvConf
	dec := yaml.NewDecoder(strings.NewReader(block))
	dec.KnownFields(true)
	if err := dec.Decode(&conf); err != nil {
		t.Fatalf("documented Tier B example does not decode: %v", err)
	}
	if conf.ServerConf == nil || conf.AppConf == nil || conf.AppConf.Security == nil {
		t.Fatalf("documented Tier B example is missing server_conf, app_conf or app_conf.security")
	}
	return conf
}

// docsThreatModelSection returns the page text under heading, up to the next
// heading of any level.
func docsThreatModelSection(t *testing.T, heading string) string {
	t.Helper()

	_, after, found := strings.Cut(docsThreatModelPage(t), "\n"+heading+"\n")
	if !found {
		t.Fatalf("%s no longer has a %q section", docsThreatModel, heading)
	}
	var section strings.Builder
	for line := range strings.Lines(after) {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			break
		}
		section.WriteString(line)
	}
	return section.String()
}

func docsThreatModelPage(t *testing.T) string {
	t.Helper()

	page, err := os.ReadFile(docsThreatModel)
	if err != nil {
		t.Fatalf("read %s: %v", docsThreatModel, err)
	}
	return string(page)
}

func containsOrigin(origins []string, want string) bool {
	for _, origin := range origins {
		if origin == want {
			return true
		}
	}
	return false
}
