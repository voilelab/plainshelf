package main

import (
	"encoding/json/v2"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// fakeSecrets stands in for the OS keychain.
type fakeSecrets struct {
	mu sync.Mutex
	m  map[string]string
	// failDelete makes Delete refuse, as a locked keychain does.
	failDelete bool
}

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{m: map[string]string{}} }

func (f *fakeSecrets) Get(id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	secret, ok := f.m[id]
	if !ok {
		return "", errSecretNotFound
	}
	return secret, nil
}

func (f *fakeSecrets) Set(id, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[id] = secret
	return nil
}

func (f *fakeSecrets) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDelete {
		return errors.New("keychain is locked")
	}
	delete(f.m, id)
	return nil
}

// davExport serves a fresh local shelf over the read-only WebDAV export and
// returns the shelf's URL and token.
func davExport(t *testing.T) (url, token string) {
	t.Helper()
	dir := t.TempDir()
	exporter, err := server.NewApp(&server.AppConf{
		Shelves:   []*shelf.ShelfConfWithID{{ID: "default_shelf", ShelfConf: shelf.ShelfConf{LibRoot: filepath.Join(dir, "shelf")}}},
		StorePath: filepath.Join(dir, "store"),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("NewApp exporter: %v", err)
	}
	t.Cleanup(func() { exporter.Close() })
	tokenFile := filepath.Join(dir, "dav.token")
	if err := server.EnsureDAVExportToken(tokenFile); err != nil {
		t.Fatalf("EnsureDAVExportToken: %v", err)
	}
	bs, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(exporter.DAVExportHandler(tokenFile))
	t.Cleanup(srv.Close)
	return srv.URL + "/dav/default_shelf/", strings.TrimSpace(string(bs))
}

// desktopAppWithSecrets is a DesktopApp whose host: credentials resolve
// from secrets, with one local shelf already in shelves.json.
func desktopAppWithSecrets(t *testing.T, secrets *fakeSecrets) *DesktopApp {
	t.Helper()
	dir := t.TempDir()
	serverApp, err := server.NewApp(&server.AppConf{
		StorePath:   filepath.Join(dir, "store"),
		Security:    &server.SecurityConf{Mode: server.SecurityModeNone},
		Credentials: desktopCredentialResolver{store: secrets},
	})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	t.Cleanup(func() { serverApp.Close() })
	a := &DesktopApp{app: serverApp, shelvesConfigPath: filepath.Join(dir, "shelves.json"), secrets: secrets}
	if err := a.AddShelf(AddShelfParams{Name: "Local", LibRoot: filepath.Join(dir, "local")}); err != nil {
		t.Fatalf("AddShelf local: %v", err)
	}
	return a
}

func TestCheckShelfSourceTellsWrongTokenFromNoConnection(t *testing.T) {
	url, token := davExport(t)
	a := &DesktopApp{}

	check, err := a.CheckShelfSource(ShelfSourceParams{Source: shelf.SourcePlainShelf, URL: url, Secret: token})
	if err != nil || check.Code != "" {
		t.Fatalf("CheckShelfSource with the token = %+v, %v; want a shelf", check, err)
	}
	check, err = a.CheckShelfSource(ShelfSourceParams{Source: shelf.SourcePlainShelf, URL: url, Secret: "wrong"})
	if err != nil || check.Code != server.SourceCheckUnauthorized {
		t.Fatalf("CheckShelfSource with a wrong token = %+v, %v; want %s", check, err, server.SourceCheckUnauthorized)
	}

	closed := httptest.NewServer(nil)
	closedURL := closed.URL + "/dav/x/"
	closed.Close()
	check, err = a.CheckShelfSource(ShelfSourceParams{Source: shelf.SourceWebDAV, URL: closedURL})
	if err != nil || check.Code != server.SourceCheckUnavailable {
		t.Fatalf("CheckShelfSource on a closed port = %+v, %v; want %s", check, err, server.SourceCheckUnavailable)
	}
}

// A virtual shelf's secret goes to the keychain and its entry to
// virtual_shelves, under a host: reference; removing it forgets the secret.
func TestAddVirtualShelfStoresSecretInKeychainOnly(t *testing.T) {
	url, token := davExport(t)
	secrets := newFakeSecrets()
	a := desktopAppWithSecrets(t, secrets)

	err := a.AddShelf(AddShelfParams{
		Name:   "Living room",
		Remote: ShelfSourceParams{Source: shelf.SourcePlainShelf, URL: url, Secret: " " + token + " "},
	})
	if err != nil {
		t.Fatalf("AddShelf virtual: %v", err)
	}
	const id = "living-room"
	if got, _ := secrets.Get(id); got != token {
		t.Fatalf("keychain holds %q, want the token", got)
	}

	raw, err := os.ReadFile(a.shelvesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), token) {
		t.Fatalf("shelves.json carries the secret: %s", raw)
	}
	conf, err := loadDesktopShelves(a.shelvesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Shelves) != 1 || conf.Shelves[0].LibRoot == "" || conf.Shelves[0].Source != nil {
		t.Fatalf("local shelves = %+v, want the one local shelf with lib_root", conf.Shelves)
	}
	if len(conf.VirtualShelves) != 1 {
		t.Fatalf("virtual shelves = %+v, want one", conf.VirtualShelves)
	}
	entry := conf.VirtualShelves[0]
	want := shelf.SourceConf{Type: shelf.SourcePlainShelf, URL: url, Credential: "host:" + id}
	if entry.ID != id || entry.LibRoot != "" || entry.Source == nil || *entry.Source != want {
		t.Fatalf("virtual entry = %+v (source %+v), want source %+v", entry, entry.Source, want)
	}

	assertShelfReadOnly(t, a, id, true)

	details, err := a.GetShelfDetails(id)
	if err != nil {
		t.Fatalf("GetShelfDetails: %v", err)
	}
	if details.Source != shelf.SourcePlainShelf || details.URL != url || !details.ReadOnly || details.Path != "" {
		t.Fatalf("details = %+v", details)
	}
	if bs, _ := json.Marshal(details); strings.Contains(string(bs), token) {
		t.Fatalf("details carry the secret: %s", bs)
	}

	// Editing keeps the source.
	if err := a.ModifyShelf(ModifyShelfParams{ShelfID: id, Name: "Lounge"}); err != nil {
		t.Fatalf("ModifyShelf: %v", err)
	}
	conf, _ = loadDesktopShelves(a.shelvesConfigPath)
	if conf.VirtualShelves[0].Name != "Lounge" || conf.VirtualShelves[0].Source == nil {
		t.Fatalf("modified entry = %+v", conf.VirtualShelves[0])
	}

	if err := a.RemoveShelf(id); err != nil {
		t.Fatalf("RemoveShelf: %v", err)
	}
	if _, err := secrets.Get(id); err == nil {
		t.Fatal("RemoveShelf left the secret in the keychain")
	}
	if _, ok := a.app.ShelfManager().GetShelf(id); ok {
		t.Fatal("RemoveShelf left the shelf open")
	}
	conf, _ = loadDesktopShelves(a.shelvesConfigPath)
	if len(conf.VirtualShelves) != 0 || len(conf.Shelves) != 1 {
		t.Fatalf("after RemoveShelf: %+v", conf)
	}
}

func TestAddVirtualShelfRefusesWhatTheDesktopCannotOpen(t *testing.T) {
	secrets := newFakeSecrets()
	a := desktopAppWithSecrets(t, secrets)
	for _, remote := range []ShelfSourceParams{
		{Source: shelf.SourcePCloud, URL: "https://example.com/"},
		{Source: shelf.SourcePlainShelf, URL: "https://example.com/"}, // plainshelf needs a token
		{Source: shelf.SourceWebDAV, URL: "ftp://example.com/"},
	} {
		if err := a.AddShelf(AddShelfParams{Name: "Remote", Remote: remote}); err == nil {
			t.Errorf("AddShelf(%+v) succeeded", remote)
		}
	}
	if len(secrets.m) != 0 {
		t.Fatalf("a refused shelf left secrets behind: %v", secrets.m)
	}
	conf, _ := loadDesktopShelves(a.shelvesConfigPath)
	if len(conf.VirtualShelves) != 0 {
		t.Fatalf("a refused shelf was saved: %+v", conf.VirtualShelves)
	}
}

// legacyDesktopShelvesConfig is shelves.json as builds before virtual shelves
// read it.
type legacyDesktopShelvesConfig struct {
	Shelves []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		LibRoot string `json:"lib_root"`
	} `json:"shelves"`
}

// A downgraded desktop still opens every local shelf: the virtual entries are
// under a key it does not know, so it never meets an entry without lib_root.
func TestShelvesWithVirtualEntryStillOpenInAnOlderBuild(t *testing.T) {
	url, token := davExport(t)
	a := desktopAppWithSecrets(t, newFakeSecrets())
	err := a.AddShelf(AddShelfParams{
		Name:   "NAS",
		Remote: ShelfSourceParams{Source: shelf.SourceWebDAV, URL: url, Username: "reader", Secret: token},
	})
	if err != nil {
		t.Fatalf("AddShelf virtual: %v", err)
	}

	raw, err := os.ReadFile(a.shelvesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var legacy legacyDesktopShelvesConfig
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatalf("an older build cannot decode shelves.json: %v", err)
	}
	var confs []*shelf.ShelfConfWithID
	for _, e := range legacy.Shelves {
		confs = append(confs, &shelf.ShelfConfWithID{ID: e.ID, Name: e.Name, ShelfConf: shelf.ShelfConf{LibRoot: e.LibRoot}})
	}
	if len(confs) != 1 || confs[0].ID != "local" {
		t.Fatalf("an older build reads shelves %+v, want the local one", legacy.Shelves)
	}
	old, err := server.NewApp(&server.AppConf{
		Shelves:   confs,
		StorePath: filepath.Join(t.TempDir(), "store"),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatalf("an older build fails to start: %v", err)
	}
	old.Close()
}

func TestDesktopCredentialResolver(t *testing.T) {
	secrets := newFakeSecrets()
	_ = secrets.Set("nas", "s3cret")
	r := desktopCredentialResolver{store: secrets}

	if got, err := r.Resolve(t.Context(), "host:nas"); err != nil || got != "s3cret" {
		t.Fatalf("host:nas = %q, %v", got, err)
	}
	if _, err := r.Resolve(t.Context(), "host:missing"); err == nil {
		t.Fatal("host:missing resolved")
	}
	t.Setenv("PLAINSHELF_TEST_SECRET", "from-env")
	if got, err := r.Resolve(t.Context(), "env:PLAINSHELF_TEST_SECRET"); err != nil || got != "from-env" {
		t.Fatalf("env: = %q, %v; want the default resolver's answer", got, err)
	}
}

// A virtual shelf whose secret is gone is skipped at startup, not fatal.
func TestOpenVirtualShelvesSkipsOneThatFails(t *testing.T) {
	url, _ := davExport(t)
	a := desktopAppWithSecrets(t, newFakeSecrets())
	errs := openVirtualShelves(a.app, []desktopShelfEntry{{
		ID:     "gone",
		Name:   "Gone",
		Source: &shelf.SourceConf{Type: shelf.SourcePlainShelf, URL: url, Credential: "host:gone"},
	}})
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want one", errs)
	}
	if _, ok := a.app.ShelfManager().GetShelf("local"); !ok {
		t.Fatal("the local shelf is not open")
	}
}

// A keychain that refuses the delete leaves the shelf in place, so removing
// it again retries rather than orphaning the secret.
func TestRemoveVirtualShelfKeepsEntryWhenKeychainRefuses(t *testing.T) {
	url, token := davExport(t)
	secrets := newFakeSecrets()
	a := desktopAppWithSecrets(t, secrets)
	err := a.AddShelf(AddShelfParams{
		Name:   "Living room",
		Remote: ShelfSourceParams{Source: shelf.SourcePlainShelf, URL: url, Secret: token},
	})
	if err != nil {
		t.Fatalf("AddShelf virtual: %v", err)
	}
	const id = "living-room"

	secrets.failDelete = true
	if err := a.RemoveShelf(id); err == nil {
		t.Fatal("RemoveShelf succeeded with a refusing keychain")
	}
	if _, ok := a.app.ShelfManager().GetShelf(id); !ok {
		t.Fatal("the shelf was closed although its secret stayed")
	}
	if conf, _ := loadDesktopShelves(a.shelvesConfigPath); conf.find(id) == nil {
		t.Fatal("the entry was removed although its secret stayed")
	}

	secrets.failDelete = false
	if err := a.RemoveShelf(id); err != nil {
		t.Fatalf("retried RemoveShelf: %v", err)
	}
	if _, err := secrets.Get(id); err == nil {
		t.Fatal("the retry left the secret in the keychain")
	}
}
