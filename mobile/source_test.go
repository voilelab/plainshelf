package mobile

import (
	"encoding/json/v2"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// mapSecrets stands in for the native Keystore store.
type mapSecrets map[string]string

func (m mapSecrets) Secret(entryID string) (string, error) { return m[entryID], nil }

func setSecretsT(t *testing.T, store SecretStore) {
	t.Helper()
	SetSecretStore(store)
	t.Cleanup(func() { SetSecretStore(nil) })
}

// serveDAVShelf exports a shelf holding "Dune" over the read-only WebDAV
// export and returns its URL and token.
func serveDAVShelf(t *testing.T) (url, token string) {
	t.Helper()
	dir := t.TempDir()
	libRoot := filepath.Join(dir, "shelf")
	s, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: libRoot, LockMode: "none", BookCacheWriterID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	book, err := s.NewBook(shelf.FolderPath{}, "Dune")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := book.NewSource(strings.NewReader("Dune text")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatal(err)
	}
	s.Close()

	exporter, err := server.NewApp(&server.AppConf{
		Shelves:   []*shelf.ShelfConfWithID{{ID: "default_shelf", ShelfConf: shelf.ShelfConf{LibRoot: libRoot, ReadOnly: true}}},
		StorePath: filepath.Join(dir, "store"),
		Security:  &server.SecurityConf{Mode: server.SecurityModeNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { exporter.Close() })
	tokenFile := filepath.Join(dir, "dav.token")
	if err := server.EnsureDAVExportToken(tokenFile); err != nil {
		t.Fatal(err)
	}
	bs, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(exporter.DAVExportHandler(tokenFile))
	t.Cleanup(srv.Close)
	return srv.URL + "/dav/default_shelf/", strings.TrimSpace(string(bs))
}

// The token is resolved through host:<entryID> from the native store; it is
// never an argument of the open.
func TestOpenSourceShelfResolvesHostCredential(t *testing.T) {
	url, token := serveDAVShelf(t)
	setSecretsT(t, mapSecrets{"entry-1": token})
	startT(t, t.TempDir())

	if err := OpenSourceShelf("dav", "Living room", shelf.SourcePlainShelf, url, "", "entry-1", false); err != nil {
		t.Fatalf("OpenSourceShelf: %v", err)
	}
	waitReady(t)
	if titles := bookTitles(t, "dav"); !strings.Contains(titles, `"Dune"`) {
		t.Fatalf("books = %s", titles)
	}

	// A local shelf list never shows it, and it closes like a pCloud shelf.
	if out, err := ListShelves(); err != nil || strings.Contains(out, `"dav"`) {
		t.Fatalf("ListShelves = %s, %v", out, err)
	}
	if err := CloseVirtualShelf("dav"); err != nil {
		t.Fatalf("CloseVirtualShelf: %v", err)
	}
}

func TestOpenSourceShelfWithoutStoredSecret(t *testing.T) {
	url, _ := serveDAVShelf(t)
	setSecretsT(t, mapSecrets{})
	startT(t, t.TempDir())

	// A plainshelf source needs a token, so the open is refused.
	if err := OpenSourceShelf("dav", "Living room", shelf.SourcePlainShelf, url, "", "entry-1", false); err == nil {
		t.Fatal("OpenSourceShelf opened a plainshelf source with no secret")
	}
	if err := OpenSourceShelf("dav", "NAS", shelf.SourcePCloud, url, "", "entry-1", false); err == nil {
		t.Fatal("OpenSourceShelf opened a pcloud source")
	}
}

func TestCredentialResolverHost(t *testing.T) {
	setSecretsT(t, mapSecrets{"e": "s3cret"})
	r := credentialResolver{}
	if got, err := r.Resolve(t.Context(), "host:e"); err != nil || got != "s3cret" {
		t.Fatalf("host:e = %q, %v", got, err)
	}
	if _, err := r.Resolve(t.Context(), "host:missing"); err == nil {
		t.Fatal("host:missing resolved")
	}
	t.Setenv("PLAINSHELF_TEST_SECRET", "from-env")
	if got, err := r.Resolve(t.Context(), "env:PLAINSHELF_TEST_SECRET"); err != nil || got != "from-env" {
		t.Fatalf("env: = %q, %v", got, err)
	}
	SetSecretStore(nil)
	if _, err := r.Resolve(t.Context(), "host:e"); err == nil {
		t.Fatal("host:e resolved with no store installed")
	}
}

func TestCheckSourceReportsCode(t *testing.T) {
	url, token := serveDAVShelf(t)
	for secret, want := range map[string]server.SourceCheck{
		token:   {Books: 1},
		"wrong": {Code: server.SourceCheckUnauthorized},
	} {
		out, err := CheckSource(shelf.SourcePlainShelf, url, "", secret)
		if err != nil {
			t.Fatalf("CheckSource: %v", err)
		}
		var got server.SourceCheck
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("CheckSource = %s, want %+v", out, want)
		}
	}
}

func TestSourceEntryRemovalCleansOfflineScope(t *testing.T) {
	url, token := serveDAVShelf(t)
	setSecretsT(t, mapSecrets{"entry-1": token})
	dataDir := t.TempDir()
	startT(t, dataDir)
	if err := OpenSourceShelf("dav", "LAN", shelf.SourcePlainShelf, url, "", "entry-1", false); err != nil {
		t.Fatal(err)
	}
	waitReady(t)
	src := shelf.SourceConf{Type: shelf.SourcePlainShelf, URL: url}
	assertOfflineScope(t, dataDir, src.Scope(), true)
	if err := CloseVirtualShelf("dav"); err != nil {
		t.Fatal(err)
	}
	assertOfflineScope(t, dataDir, src.Scope(), true)
	// Deletion must work without the entry open or its secret available.
	SetSecretStore(nil)
	if err := RemoveSourceShelfOffline(shelf.SourcePlainShelf, url, ""); err != nil {
		t.Fatal(err)
	}
	assertOfflineScope(t, dataDir, src.Scope(), false)
	if err := RemoveSourceShelfOffline(shelf.SourcePlainShelf, "", ""); err == nil {
		t.Fatal("empty source URL accepted")
	}
}

// Source type is not part of the offline scope, while account is. Hosts must
// compare the engine's scope rather than a TS cache key when retaining aliases.
func TestSourceShelfOfflineScope(t *testing.T) {
	for _, sourceType := range []string{shelf.SourcePlainShelf, shelf.SourceWebDAV} {
		scope, err := SourceShelfOfflineScope(sourceType, " https://NAS/books ", "reader")
		if err != nil || scope != "https://reader@nas/books/" {
			t.Fatalf("scope = %q, %v", scope, err)
		}
		other, err := SourceShelfOfflineScope(sourceType, "https://nas/books/", "other")
		if err != nil || other == scope {
			t.Fatalf("account scope = %q, %v", other, err)
		}
	}
	if _, err := SourceShelfOfflineScope(shelf.SourcePCloud, "https://nas/books/", ""); err == nil {
		t.Fatal("unsupported source type accepted")
	}
	if _, err := SourceShelfOfflineScope(shelf.SourceWebDAV, "", ""); err == nil {
		t.Fatal("empty source accepted")
	}
}
