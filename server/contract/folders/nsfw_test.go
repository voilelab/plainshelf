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

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

func TestAPINSFWFoldersLeaveTheTreeWithTheirBooks(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	folders := func() []string {
		paths := []string{}
		for _, folder := range apitest.GetJSON[[]shelf.FolderPath](t, s.Env, apitest.ShelfURL("folders")) {
			paths = append(paths, folder.String())
		}
		return paths
	}
	assertListed := func(got []string, want bool, folders ...string) {
		t.Helper()
		for _, folder := range folders {
			if slices.Contains(got, folder) != want {
				t.Errorf("folders = %v, want %q listed = %v", got, folder, want)
			}
		}
	}

	got := folders()

	assertListed(got, true, "Fiction", "Fiction/Classics", apitest.NSFWEmptyFolder)

	assertListed(got, false, apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder)

	apitest.SetShowNSFW(t, s.Env, true)
	assertListed(folders(), true, apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder)
}

func trashAllOf(t *testing.T, s apitest.NSFWShelf, bookIDs ...string) {
	t.Helper()

	apitest.SetShowNSFW(t, s.Env, true)
	for _, bookID := range bookIDs {
		apitest.AssertStatus(t, s.Env.Post(apitest.BookURL(bookID, "trash"), nil), http.StatusNoContent)
	}
	apitest.SetShowNSFW(t, s.Env, false)
}

func TestAPINSFWBooksAreAbsentFromTheTrash(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	trashAllOf(t, s, s.Visible, s.FolderHidden, s.BookHidden)

	rec := s.Env.Get(apitest.TrashBooksURL())
	apitest.AssertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	for _, disclosure := range []string{
		"FolderHidden", "BookHidden", apitest.NSFWMarkedFolder, apitest.NSFWFlaggedFolder,
	} {
		if strings.Contains(body, disclosure) {
			t.Errorf("trash body contains %q, want it withheld: %s", disclosure, body)
		}
	}

	trashedIDs := func() []string {
		ids := []string{}
		for _, book := range apitest.GetJSON[[]server.TrashedBook](t, s.Env, apitest.TrashBooksURL()) {
			ids = append(ids, book.ID)
		}
		return ids
	}
	apitest.AssertBookIDs(t, trashedIDs(), s.Visible)

	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertBookIDs(t, trashedIDs(), s.Visible, s.FolderHidden, s.BookHidden)
}

func TestAPINSFWTrashListingMarksItselfPartial(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	partial := func() string {
		rec := s.Env.Get(apitest.TrashBooksURL())
		apitest.AssertStatus(t, rec, http.StatusOK)
		return rec.Header().Get(server.TrashListingPartialHeader)
	}

	if got := partial(); got != "" {
		t.Errorf("header on an empty trash = %q, want it absent", got)
	}
	trashAllOf(t, s, s.Visible)
	if got := partial(); got != "" {
		t.Errorf("header with nothing withheld = %q, want it absent", got)
	}

	trashAllOf(t, s, s.BookHidden)
	if got := partial(); got != "true" {
		t.Errorf("header with a marked book withheld = %q, want \"true\"", got)
	}

	apitest.SetShowNSFW(t, s.Env, true)
	if got := partial(); got != "" {
		t.Errorf("header with show_nsfw on = %q, want it absent", got)
	}
}

func TestAPINSFWTrashedBooksNotFound(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	trashAllOf(t, s, s.FolderHidden, s.BookHidden)

	routes := []struct {
		method string
		elem   []string
	}{
		{method: http.MethodPost, elem: []string{"restore"}},
		{method: http.MethodDelete},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+strings.Join(tc.elem, "/"), func(t *testing.T) {
			for _, bookID := range append([]string{"no_such_book"}, s.Hidden()...) {
				rec := s.Env.Request(tc.method, apitest.TrashBooksURL(append([]string{bookID}, tc.elem...)...), nil)
				apitest.AssertErrorEnvelope(t, rec, http.StatusNotFound,
					"TRASHED_BOOK_NOT_FOUND", "trashed book not found")
			}
		})
	}

	apitest.SetShowNSFW(t, s.Env, true)
	for _, bookID := range s.Hidden() {
		apitest.AssertStatus(t, s.Env.Post(apitest.TrashBooksURL(bookID, "restore"), nil), http.StatusNoContent)
	}
}

func TestAPINSFWEmptyTrashStillErasesEverything(t *testing.T) {
	s := apitest.NewNSFWShelf(t)
	trashAllOf(t, s, s.Visible, s.FolderHidden, s.BookHidden)

	accepted := apitest.EmptyTrash(t, s.Env, http.StatusAccepted)
	if chain := apitest.WaitForTaskChain(t, s.Env, accepted.TaskChainID); chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}

	apitest.SetShowNSFW(t, s.Env, true)
	if trashed := apitest.GetJSON[[]server.TrashedBook](t, s.Env, apitest.TrashBooksURL()); len(trashed) != 0 {
		t.Errorf("trashed books after empty = %+v, want none", trashed)
	}
}

type nsfwRevealConflictBody struct {
	Error       string `json:"error"`
	Message     string `json:"message"`
	HiddenBooks int    `json:"hidden_books"`
}

const nsfwRevealConflictKind = "nsfw_reveal_requires_confirmation"

func assertRevealConflict(t *testing.T, rec *httptest.ResponseRecorder, wantHidden int) {
	t.Helper()

	apitest.AssertStatus(t, rec, http.StatusConflict)
	body := apitest.DecodeJSON[nsfwRevealConflictBody](t, rec)
	if body.Error != nsfwRevealConflictKind {
		t.Errorf("error = %q, want %q", body.Error, nsfwRevealConflictKind)
	}
	if body.HiddenBooks != wantHidden {
		t.Errorf("hidden_books = %d, want %d", body.HiddenBooks, wantHidden)
	}
	if body.Message == "" {
		t.Error("message is empty, want the explanation the dialog shows")
	}
}

func confirmed(url string) string { return url + "?confirm=1" }

const nsfwMoveBody = `{"folder":["Fiction"],"target_folder":["Archive"]}`

func newNSFWShelfWithArchive(t *testing.T) apitest.NSFWShelf {
	t.Helper()

	s := apitest.NewNSFWShelf(t)
	apitest.AssertStatus(t, s.Env.Post(apitest.ShelfURL("folders", "Archive"), nil), http.StatusNoContent)
	return s
}

func TestAPINSFWFolderMoveAsksBeforeUnhiding(t *testing.T) {
	s := newNSFWShelfWithArchive(t)
	moves := apitest.ShelfURL("folder-moves")

	assertRevealConflict(t, s.Env.Post(moves, strings.NewReader(nsfwMoveBody)), 1)

	folders := apitest.GetJSON[[]shelf.FolderPath](t, s.Env, apitest.ShelfURL("folders"))
	if !slices.ContainsFunc(folders, func(f shelf.FolderPath) bool { return f.String() == "Fiction" }) {
		t.Fatalf("folders = %v, want Fiction still at the root", folders)
	}
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)

	apitest.AssertStatus(t, s.Env.Post(confirmed(moves), strings.NewReader(nsfwMoveBody)), http.StatusNoContent)

	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic, s.FolderHidden)
}

func TestAPINSFWFolderMoveDoesNotAskWhileShowNSFWIsOn(t *testing.T) {
	s := newNSFWShelfWithArchive(t)
	apitest.SetShowNSFW(t, s.Env, true)

	apitest.AssertStatus(t,
		s.Env.Post(apitest.ShelfURL("folder-moves"), strings.NewReader(nsfwMoveBody)),
		http.StatusNoContent)
}

func TestAPINSFWFolderMoveWithoutMarks(t *testing.T) {
	s := newNSFWShelfWithArchive(t)
	moves := apitest.ShelfURL("folder-moves")

	for _, folder := range []string{"Fiction/Classics", apitest.NSFWFlaggedFolder} {
		body := `{"folder":["Fiction","` + strings.Split(folder, "/")[1] + `"],"target_folder":["Archive"]}`
		apitest.AssertStatus(t, s.Env.Post(moves, strings.NewReader(body)), http.StatusNoContent)
	}

	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)
}

func TestAPINSFWFolderRenameAsksBeforeUnhiding(t *testing.T) {
	renames := map[string]string{
		"the marked folder itself": apitest.NSFWMarkedFolder,
		"an ancestor of it":        "Fiction",
	}

	for name, folder := range renames {
		t.Run(name, func(t *testing.T) {
			s := apitest.NewNSFWShelf(t)
			url := apitest.ShelfURL("folders", folder)
			body := func() *strings.Reader { return strings.NewReader(`{"name":"General"}`) }

			assertRevealConflict(t, s.Env.Patch(url, body()), 1)
			apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)

			apitest.AssertStatus(t, s.Env.Patch(confirmed(url), body()), http.StatusNoContent)
			apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic, s.FolderHidden)
		})
	}
}

func TestAPINSFWFolderRenameAsksForAnEmptyMarkedFolder(t *testing.T) {
	s := apitest.NewNSFWShelf(t)

	apitest.SetShowNSFW(t, s.Env, true)
	apitest.AssertStatus(t, s.Env.Post(apitest.BookURL(s.FolderHidden, "trash"), nil), http.StatusNoContent)
	apitest.SetShowNSFW(t, s.Env, false)

	assertRevealConflict(t, s.Env.Patch(
		apitest.ShelfURL("folders", apitest.NSFWMarkedFolder),
		strings.NewReader(`{"name":"General"}`)), 0)
}

func TestAPINSFWFolderTransferAsksBeforeUnhiding(t *testing.T) {
	for _, mode := range []string{"copy", "move"} {
		t.Run(mode, func(t *testing.T) {
			s := apitest.NewNSFWShelf(t, apitest.WithSecondShelf(t.TempDir()))
			transfers := apitest.FolderTransfersURL()
			body := `{"mode":"` + mode + `","source_folder":["Fiction"],` +
				`"target_shelf":"` + apitest.SecondShelfID + `","target_folder":["Imported"]}`

			assertRevealConflict(t, s.Env.Post(transfers, strings.NewReader(body)), 1)
			if books := apitest.GetJSON[[]server.Book](t, s.Env, apitest.SecondShelfBooksURL()); len(books) != 0 {
				t.Fatalf("target books after the refusal = %#v, want none", books)
			}

			accepted := apitest.SubmitTaskChain(t, s.Env, confirmed(transfers), []byte(body), http.StatusAccepted)
			if chain := apitest.WaitForTaskChain(t, s.Env, accepted.TaskChainID); chain.Status != "completed" {
				t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
			}
		})
	}
}

func TestAPINSFWFolderTransferWithoutMarks(t *testing.T) {
	s := apitest.NewNSFWShelf(t, apitest.WithSecondShelf(t.TempDir()))

	apitest.SubmitTaskChain(t, s.Env, apitest.FolderTransfersURL(), []byte(`{
		"mode": "copy",
		"source_folder": ["Fiction", "Classics"],
		"target_shelf": "`+apitest.SecondShelfID+`",
		"target_folder": ["Imported"]
	}`), http.StatusAccepted)
}

func TestAPINSFWFolderMoveKeepsBooksHidden(t *testing.T) {
	s := newNSFWShelfWithArchive(t)

	apitest.EditBookMetaFile(t, s.Env, s.FolderHidden, map[string]any{"nsfw": true})
	apitest.AssertStatus(t, s.Env.Post(apitest.ScansURL(), nil), http.StatusOK)

	apitest.AssertStatus(t,
		s.Env.Post(apitest.ShelfURL("folder-moves"), strings.NewReader(nsfwMoveBody)),
		http.StatusNoContent)
	apitest.AssertBookIDs(t, apitest.ListedBookIDs(t, s.Env), s.Visible, s.Classic)
}

func TestAPINSFWFolderRenameConflict(t *testing.T) {
	s := newNSFWShelfWithArchive(t)

	rec := s.Env.Patch(apitest.ShelfURL("folders", "Fiction"), strings.NewReader(`{"name":"Archive"}`))
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "UNKNOWN", "failed to rename folder")
}

func TestAPINSFWFolderMoveWithUnscannedMark(t *testing.T) {
	const marked = "Fiction/Secret"

	libRoot := t.TempDir()
	shelfJSON := `{"schema_version":1,"content":{"nsfw_folders":[{"path":"` + marked + `"}]}}`
	if err := os.WriteFile(filepath.Join(libRoot, "shelf.json"), []byte(shelfJSON), 0o644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}

	env := apitest.New(t, apitest.WithLibRoot(libRoot))
	apitest.ImportTextBook(t, env, "Classic", "Fiction/Classics", "classic.txt", "a visible book")
	apitest.AssertStatus(t, env.Post(apitest.ShelfURL("folders", "Archive"), nil), http.StatusNoContent)
	apitest.AssertStatus(t, env.Post(apitest.ScansURL(), nil), http.StatusOK)

	if err := os.MkdirAll(filepath.Join(libRoot, "books", filepath.FromSlash(marked)), 0o755); err != nil {
		t.Fatalf("create the folder behind the shelf's back: %v", err)
	}

	assertRevealConflict(t, env.Post(apitest.ShelfURL("folder-moves"), strings.NewReader(nsfwMoveBody)), 0)
}

func TestAPINSFWFolderChangeReadOnly(t *testing.T) {
	libRoot := t.TempDir()
	shelfJSON := `{"schema_version":1,"content":{"nsfw_folders":[{"path":"Fiction/Secret"}]}}`
	if err := os.WriteFile(filepath.Join(libRoot, "shelf.json"), []byte(shelfJSON), 0o644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}
	for _, folder := range []string{"Archive", "Fiction/Classics", "Fiction/Secret"} {
		if err := os.MkdirAll(filepath.Join(libRoot, "books", filepath.FromSlash(folder)), 0o755); err != nil {
			t.Fatalf("create %s: %v", folder, err)
		}
	}

	env := apitest.New(t, apitest.WithLibRoot(libRoot), apitest.WithReadOnlyShelf())

	changes := []struct {
		name string
		rec  func() *httptest.ResponseRecorder
	}{
		{name: "move", rec: func() *httptest.ResponseRecorder {
			return env.Post(apitest.ShelfURL("folder-moves"), strings.NewReader(nsfwMoveBody))
		}},
		{name: "rename", rec: func() *httptest.ResponseRecorder {
			return env.Patch(apitest.ShelfURL("folders", "Fiction"), strings.NewReader(`{"name":"Library"}`))
		}},
	}

	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			apitest.AssertErrorEnvelope(t, tc.rec(), http.StatusConflict, "SHELF_READ_ONLY",
				"shelf is opened read-only; this PlainShelf instance cannot modify it")
		})
	}

	malformed := []struct {
		name string
		rec  func() *httptest.ResponseRecorder
	}{
		{name: "move", rec: func() *httptest.ResponseRecorder {
			return env.Post(apitest.ShelfURL("folder-moves"),
				strings.NewReader(`{"folder":[".."],"target_folder":["Archive"]}`))
		}},
		{name: "rename", rec: func() *httptest.ResponseRecorder {
			return env.Patch(apitest.ShelfURL("folders", "Fiction"), strings.NewReader(`{"name":".."}`))
		}},
	}

	for _, tc := range malformed {
		t.Run("malformed "+tc.name, func(t *testing.T) {
			apitest.AssertErrorEnvelope(t, tc.rec(), http.StatusBadRequest,
				"INVALID_FOLDER", "invalid folder name")
		})
	}
}
