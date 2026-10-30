package shelves_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

func shelfInfoByID(t *testing.T, env *apitest.Env) map[string]server.ShelfInfo {
	t.Helper()

	infos := map[string]server.ShelfInfo{}
	for _, info := range apitest.GetJSON[[]server.ShelfInfo](t, env, "/api/shelves") {
		infos[info.ID] = info
	}
	return infos
}

func TestAPIShelvesReportCapabilities(t *testing.T) {
	env := apitest.New(t)
	libRoot, _ := apitest.SeedVirtualSource(t, "Dune")
	env.AddVirtualShelf(t, libRoot)

	infos := shelfInfoByID(t, env)

	local := infos[apitest.DefaultShelfID]
	wantLocal := server.ShelfCapabilities{Rescan: true, Download: false, Fingerprint: true, CharCount: true}
	if local.Kind != "local" || local.Capabilities != wantLocal || local.Offline != nil {
		t.Errorf("local shelf = %+v, want kind local, %+v, no offline", local, wantLocal)
	}

	virtual := infos[apitest.VirtualShelfID]
	wantVirtual := server.ShelfCapabilities{Rescan: true, Download: true, Fingerprint: false, CharCount: false}
	if virtual.Kind != "virtual" || !virtual.ReadOnly || virtual.Capabilities != wantVirtual {
		t.Errorf("virtual shelf = %+v, want kind virtual, read-only, %+v", virtual, wantVirtual)
	}
	if virtual.Offline == nil || virtual.Offline.SnapshotAt.IsZero() {
		t.Errorf("virtual shelf offline = %+v, want a snapshot time", virtual.Offline)
	}

	rec := env.Get("/api/shelves")
	for _, key := range []string{`"kind"`, `"capabilities"`, `"rescan"`, `"download"`, `"fingerprint"`, `"char_count"`, `"offline"`, `"snapshot_at"`} {
		if !strings.Contains(rec.Body.String(), key) {
			t.Errorf("GET /api/shelves body lacks %s: %s", key, rec.Body.String())
		}
	}
}

func TestAPIDownloadBookContract(t *testing.T) {
	env := apitest.New(t)
	libRoot, ids := apitest.SeedVirtualSource(t, "Dune", "Solaris")
	env.AddVirtualShelf(t, libRoot)
	dune := ids["Dune"]

	rec := env.Put(apitest.BookDownloadURL(dune), nil)
	apitest.AssertStatus(t, rec, http.StatusOK)
	download := apitest.DecodeJSON[server.DownloadResponse](t, rec)
	if download.Book.Meta.ID != dune || download.Book.DownloadState != shelf.DownloadDownloaded || download.SizeBytes == 0 || download.DownloadedAt.IsZero() {
		t.Errorf("download response = %+v", download)
	}

	book := apitest.GetJSON[server.Book](t, env, apitest.VirtualBookURL(dune))
	if book.DownloadState != shelf.DownloadDownloaded || book.DownloadedAt == nil {
		t.Errorf("downloaded book reports %q at %v", book.DownloadState, book.DownloadedAt)
	}
	other := apitest.GetJSON[server.Book](t, env, apitest.VirtualBookURL(ids["Solaris"]))
	if other.DownloadState != shelf.DownloadNotDownloaded || other.DownloadedAt != nil {
		t.Errorf("book without a copy reports %q at %v", other.DownloadState, other.DownloadedAt)
	}

	list := apitest.GetJSON[server.DownloadsResponse](t, env, apitest.DownloadsURL())
	if len(list.Books) != 1 || list.Books[0].Book.Meta.ID != dune || list.TotalBytes != download.SizeBytes {
		t.Errorf("downloads = %+v, want only %s totalling %d bytes", list, dune, download.SizeBytes)
	}

	rec = env.Delete(apitest.BookDownloadURL(dune))
	apitest.AssertStatus(t, rec, http.StatusNoContent)
	book = apitest.GetJSON[server.Book](t, env, apitest.VirtualBookURL(dune))
	if book.DownloadState != shelf.DownloadNotDownloaded || book.DownloadedAt != nil {
		t.Errorf("after removal the book reports %q at %v", book.DownloadState, book.DownloadedAt)
	}
	if list := apitest.GetJSON[server.DownloadsResponse](t, env, apitest.DownloadsURL()); len(list.Books) != 0 || list.TotalBytes != 0 {
		t.Errorf("downloads after removal = %+v", list)
	}
}

func TestRemoveOfflineScopeInUse(t *testing.T) {
	env := apitest.New(t)
	libRoot, ids := apitest.SeedVirtualSource(t, "Dune")
	env.AddVirtualShelf(t, libRoot)
	scope := "test://" + libRoot

	apitest.AssertStatus(t, env.Put(apitest.BookDownloadURL(ids["Dune"]), nil), http.StatusOK)

	if err := env.App.RemoveOfflineScope(scope); !errors.Is(err, server.ErrOfflineScopeInUse) {
		t.Fatalf("RemoveOfflineScope with the shelf open = %v, want %v", err, server.ErrOfflineScopeInUse)
	}
	if list := apitest.GetJSON[server.DownloadsResponse](t, env, apitest.DownloadsURL()); len(list.Books) != 1 {
		t.Fatalf("downloads after a refused removal = %+v, want the copy kept", list)
	}

	if err := env.App.RemoveShelf(apitest.VirtualShelfID); err != nil {
		t.Fatalf("RemoveShelf: %v", err)
	}
	if err := env.App.RemoveOfflineScope(scope); err != nil {
		t.Fatalf("RemoveOfflineScope after closing the shelf: %v", err)
	}
	env.AddVirtualShelf(t, libRoot)
	if list := apitest.GetJSON[server.DownloadsResponse](t, env, apitest.DownloadsURL()); len(list.Books) != 0 {
		t.Errorf("downloads after removing the scope = %+v, want none", list)
	}
}

func TestAPILocalBooksHaveNoDownloadState(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Dune", "", "dune.txt", "Dune text")

	rec := env.Get(apitest.BookURL(book.Meta.ID))
	apitest.AssertStatus(t, rec, http.StatusOK)
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["download_state"]; ok {
		t.Errorf("local book carries download_state: %s", rec.Body.String())
	}

	rec = env.Put(apitest.ShelfURL("books", book.Meta.ID, "download"), nil)
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "DOWNLOAD_UNSUPPORTED", "this shelf is already local; only a virtual shelf keeps offline copies")
}

func TestAPIOfflineVirtualShelfContract(t *testing.T) {
	env := apitest.New(t)
	libRoot, ids := apitest.SeedVirtualSource(t, "Dune", "Solaris")
	source := env.AddVirtualShelf(t, libRoot)
	apitest.AssertStatus(t, env.Put(apitest.BookDownloadURL(ids["Dune"]), nil), http.StatusOK)

	source.SetOffline(true)

	rec := env.Get(apitest.VirtualBookURL(ids["Dune"], "content"))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if rec.Body.String() != "Dune text" {
		t.Errorf("downloaded book offline = %q", rec.Body.String())
	}

	rec = env.Get(apitest.VirtualBookURL(ids["Solaris"], "content"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusServiceUnavailable, "BOOK_NOT_DOWNLOADED", "the shelf source is unavailable and this book has not been downloaded")

	if books := apitest.GetJSON[[]server.Book](t, env, apitest.ShelfIDURL(apitest.VirtualShelfID, "books")); len(books) != 2 {
		t.Errorf("offline listing holds %d books, want 2", len(books))
	}

	rec = env.Put(apitest.BookDownloadURL(ids["Solaris"]), nil)
	apitest.AssertErrorEnvelope(t, rec, http.StatusServiceUnavailable, "SOURCE_UNAVAILABLE", "the shelf source is unavailable")
}

func TestAPIDownloadUnderReadOnlyServer(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyServer())
	libRoot, ids := apitest.SeedVirtualSource(t, "Dune")
	env.AddVirtualShelf(t, libRoot)
	dune := ids["Dune"]

	apitest.AssertStatus(t, env.Put(apitest.BookDownloadURL(dune), nil), http.StatusOK)
	apitest.AssertStatus(t, env.Delete(apitest.BookDownloadURL(dune)), http.StatusNoContent)
	apitest.AssertStatus(t, env.Patch(apitest.VirtualBookURL(dune), strings.NewReader(`{"star": 3}`)), http.StatusForbidden)
}

func TestAPIDownloadNeedsLocalTokenContract(t *testing.T) {
	env := apitest.New(t, apitest.WithSecurity(apitest.LocalTokenSecurity()))
	libRoot, ids := apitest.SeedVirtualSource(t, "Dune")
	env.AddVirtualShelf(t, libRoot)
	url := apitest.BookDownloadURL(ids["Dune"])

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := env.DoRaw(httptest.NewRequest(method, url, nil))
		apitest.AssertStatus(t, rec, http.StatusUnauthorized)
	}
	apitest.AssertStatus(t, env.Put(url, nil), http.StatusOK)
}
