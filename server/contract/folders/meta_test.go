package folders_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

type folderMetaResponse struct {
	Icons   []string `json:"icons"`
	Folders map[string]struct {
		Icon     string `json:"icon"`
		Version  string `json:"version"`
		ReadOnly bool   `json:"read_only"`
	} `json:"folders"`
}

func getFolderMeta(t *testing.T, env *apitest.Env) folderMetaResponse {
	t.Helper()
	return apitest.GetJSON[folderMetaResponse](t, env, apitest.ShelfURL("folder-meta"))
}

func putFolderIcon(env *apitest.Env, folder, icon, version string) *httptest.ResponseRecorder {
	body := `{"icon":"` + icon + `","version":"` + version + `"}`
	return env.Put(apitest.ShelfURL("folder-meta", folder), strings.NewReader(body))
}

func TestAPIFolderMetaRoundTripContract(t *testing.T) {
	env := apitest.New(t)
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", "Fiction"), nil), http.StatusNoContent)
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", "Fiction", "Classics"), nil), http.StatusNoContent)

	got := getFolderMeta(t, env)
	if !slices.Equal(got.Icons, shelf.FolderIcons()) || len(got.Folders) != 0 {
		t.Fatalf("fresh shelf = %+v, want every icon and no folder", got)
	}

	apitest.AssertStatus(t, putFolderIcon(env, "Fiction/Classics", "rocket", ""), http.StatusNoContent)
	got = getFolderMeta(t, env)
	classics, ok := got.Folders["Fiction/Classics"]
	if !ok || classics.Icon != "rocket" || classics.Version == "" || classics.ReadOnly || len(got.Folders) != 1 {
		t.Fatalf("after setting an icon = %+v", got.Folders)
	}

	folders := apitest.GetJSON[[]shelf.FolderPath](t, env, apitest.ShelfURL("folders"))
	if len(folders) != 3 {
		t.Fatalf("GET .../folders = %v, want the three paths unchanged", folders)
	}

	apitest.AssertStatus(t, putFolderIcon(env, "Fiction/Classics", "", classics.Version), http.StatusNoContent)
	if got := getFolderMeta(t, env); len(got.Folders) != 0 {
		t.Fatalf("after clearing = %+v", got.Folders)
	}
}

func TestAPIFolderMetaRefusalsContract(t *testing.T) {
	env := apitest.New(t)
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", "Fiction"), nil), http.StatusNoContent)

	apitest.AssertErrorEnvelope(t, putFolderIcon(env, "Fiction", "not-an-icon", ""), http.StatusBadRequest,
		"INVALID_FOLDER_ICON", "folder icon is not one this server offers; see icons in GET .../folder-meta")
	apitest.AssertErrorEnvelope(t, putFolderIcon(env, "Missing", "book", ""), http.StatusNotFound,
		"FOLDER_NOT_FOUND", "folder not found")

	apitest.AssertStatus(t, putFolderIcon(env, "Fiction", "book", ""), http.StatusNoContent)
	stale := getFolderMeta(t, env).Folders["Fiction"].Version
	metaPath := filepath.Join(env.LibRoot, "books", "Fiction", shelf.FolderMetaFile)
	if err := os.WriteFile(metaPath, []byte(`{"icon":"moon"}`), 0o600); err != nil {
		t.Fatalf("write folder.json: %v", err)
	}
	apitest.AssertErrorEnvelope(t, putFolderIcon(env, "Fiction", "star", stale), http.StatusConflict,
		"FOLDER_META_CONFLICT", "folder.json changed since it was read; read it again and retry")

	fresh := getFolderMeta(t, env).Folders["Fiction"]
	if fresh.Icon != "moon" {
		t.Fatalf("after the conflict = %+v, want the file on disk", fresh)
	}
	apitest.AssertStatus(t, putFolderIcon(env, "Fiction", "star", fresh.Version), http.StatusNoContent)

	broken := `{"icon":`
	if err := os.WriteFile(metaPath, []byte(broken), 0o600); err != nil {
		t.Fatalf("write folder.json: %v", err)
	}
	apitest.AssertStatus(t, env.Post(apitest.ScansURL(), nil), http.StatusOK)
	meta := getFolderMeta(t, env).Folders["Fiction"]
	if !meta.ReadOnly || meta.Icon != "" {
		t.Fatalf("a broken folder.json listed as %+v", meta)
	}
	apitest.AssertErrorEnvelope(t, putFolderIcon(env, "Fiction", "star", meta.Version), http.StatusConflict,
		"FOLDER_META_NOT_WRITABLE", "folder.json cannot be read safely by this server, so it was left unchanged; repair or remove the file")
	if onDisk, _ := os.ReadFile(metaPath); string(onDisk) != broken {
		t.Fatalf("a refused write changed folder.json to %q", onDisk)
	}

	apitest.AssertStatus(t, env.Put(apitest.ShelfURL("folder-meta", "%20"), strings.NewReader(`{"icon":"book"}`)), http.StatusBadRequest)
}

func TestAPIFolderMetaWriteIsGatedContract(t *testing.T) {
	env := apitest.New(t)
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", "Fiction"), nil), http.StatusNoContent)

	apitest.AssertMutationGated(t, env, http.MethodPut, apitest.ShelfURL("folder-meta", "Fiction"), []byte(`{"icon":"book"}`))
}

func TestAPIFolderMetaOnAReadOnlyShelfContract(t *testing.T) {
	libRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(libRoot, "books", "Fiction"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"icon":"book"}`
	if err := os.WriteFile(filepath.Join(libRoot, "books", "Fiction", shelf.FolderMetaFile), []byte(content), 0o600); err != nil {
		t.Fatalf("write folder.json: %v", err)
	}
	env := apitest.New(t, apitest.WithLibRoot(libRoot), apitest.WithReadOnlyShelf())

	if got := getFolderMeta(t, env).Folders["Fiction"].Icon; got != "book" {
		t.Fatalf("read-only shelf listed icon %q, want book", got)
	}
	rec := putFolderIcon(env, "Fiction", "star", "")
	apitest.AssertStatus(t, rec, http.StatusConflict)
	if code := apitest.DecodeJSON[apitest.ErrorEnvelope](t, rec).Error.Code; code != "SHELF_READ_ONLY" {
		t.Fatalf("error code = %q, want SHELF_READ_ONLY", code)
	}
}

// Metadata leaves the listing with its folder: an icon on a hidden folder would
// name what the filter hides, and a write to one answers as if it did not exist.
func TestAPIFolderMetaFollowsFolderVisibilityContract(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	apitest.SetShowNSFW(t, s.Env, true)
	for _, folder := range []string{"Fiction", apitest.NSFWMarkedFolder} {
		apitest.AssertStatus(t, putFolderIcon(s.Env, folder, "book", ""), http.StatusNoContent)
	}
	hiddenVersion := getFolderMeta(t, s.Env).Folders[apitest.NSFWMarkedFolder].Version

	apitest.SetShowNSFW(t, s.Env, false)
	got := getFolderMeta(t, s.Env)
	if _, leaked := got.Folders[apitest.NSFWMarkedFolder]; leaked {
		t.Fatalf("hidden folder's metadata is listed: %+v", got.Folders)
	}
	if _, ok := got.Folders["Fiction"]; !ok {
		t.Fatalf("visible folder's metadata is missing: %+v", got.Folders)
	}
	apitest.AssertErrorEnvelope(t, putFolderIcon(s.Env, apitest.NSFWMarkedFolder, "star", hiddenVersion), http.StatusNotFound,
		"FOLDER_NOT_FOUND", "folder not found")
}

func TestAPIFolderMetaReadRespectsLocalTokenContract(t *testing.T) {
	security := apitest.LocalTokenSecurity()
	security.ProtectRead = true
	env := apitest.New(t, apitest.WithSecurity(security))
	url := apitest.ShelfURL("folder-meta")
	apitest.AssertStatus(t, env.Get(url), http.StatusUnauthorized)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set(env.App.SecurityTokenHeader(), env.App.SecurityToken())
	apitest.AssertStatus(t, env.DoRaw(req), http.StatusOK)
}

func TestAPIFolderSortKeyContract(t *testing.T) {
	env := apitest.New(t)
	for _, name := range []string{"A", "B"} {
		apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", name), nil), http.StatusNoContent)
	}
	apitest.AssertStatus(t, putFolderIcon(env, "B", "moon", ""), http.StatusNoContent)
	version := getFolderMeta(t, env).Folders["B"].Version
	body := `{"sort_key":0,"version":"` + version + `"}`
	apitest.AssertMutationGated(t, env, http.MethodPut, apitest.ShelfURL("folder-meta", "B"), []byte(body))
	apitest.AssertStatus(t, env.Put(apitest.ShelfURL("folder-meta", "B"), strings.NewReader(body)), http.StatusNoContent)
	result := apitest.GetJSON[struct {
		Folders map[string]struct {
			SortKey *float64 `json:"sort_key"`
			Icon    string   `json:"icon"`
		} `json:"folders"`
	}](t, env, apitest.ShelfURL("folder-meta"))
	if result.Folders["B"].SortKey == nil || *result.Folders["B"].SortKey != 0 || result.Folders["B"].Icon != "moon" {
		t.Fatalf("metadata: %+v", result)
	}
	folders := apitest.GetJSON[[]shelf.FolderPath](t, env, apitest.ShelfURL("folders"))
	if len(folders) != 3 || folders[1].String() != "B" || folders[2].String() != "A" {
		t.Fatal(folders)
	}
	apitest.AssertStatus(t, env.Put(apitest.ShelfURL("folder-meta", "A"), strings.NewReader(`{"sort_key":"bad"}`)), http.StatusBadRequest)
}

func TestAPIFolderMetaRejectsAmbiguousSort(t *testing.T) {
	env := apitest.New(t)
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", "A"), nil), http.StatusNoContent)
	apitest.AssertStatus(t, putFolderIcon(env, "A", "moon", ""), http.StatusNoContent)
	before := getFolderMeta(t, env).Folders["A"]
	for _, fields := range []string{`"sort_key":null`, `"sort_key":3,"icon":"star"`, `"sort_key":3,"icon":null`} {
		body := `{` + fields + `,"version":"` + before.Version + `"}`
		apitest.AssertStatus(t, env.Put(apitest.ShelfURL("folder-meta", "A"), strings.NewReader(body)), http.StatusBadRequest)
		if after := getFolderMeta(t, env).Folders["A"]; after != before {
			t.Fatalf("refused write changed metadata: %+v", after)
		}
	}
}
