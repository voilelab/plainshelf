package server

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/shelf"
)

// The show_nsfw fixture. One shelf marks Fiction/Adult in its shelf.json, and
// the tree below covers every way a book or a folder can end up hidden, plus the
// three that must not be:
//
//	books/
//	├─ Visible.bookpkg                 plainly visible, top level
//	└─ Fiction/
//	   ├─ Classics/Classic.bookpkg     visible, so Fiction survives too
//	   ├─ Adult/FolderHidden.bookpkg   hidden by the shelf.json folder rule
//	   ├─ Flagged/BookHidden.bookpkg   hidden by its own book.json nsfw
//	   └─ Empty/                       no books at all, and must stay listed
//
// Fiction/Flagged is the case a folder rule cannot cover: the folder is not
// marked, so it is only empty once its one book is filtered out.
const (
	nsfwMarkedFolder  = "Fiction/Adult"
	nsfwFlaggedFolder = "Fiction/Flagged"
	nsfwEmptyFolder   = "Fiction/Empty"
)

// nsfwShelfConfig marks one folder subtree. Written before the app opens the
// shelf, because shelf.json is read once at open.
const nsfwShelfConfig = `{"schema_version":1,"content":{"nsfw_folders":[{"path":"` + nsfwMarkedFolder + `"}]}}`

type nsfwEnv struct {
	app       *App
	libRoot   string
	shelfData *shelf.ShelfData

	// The book IDs, named for why each one is or is not served.
	visible      string
	classic      string
	folderHidden string
	bookHidden   string
}

// hiddenIDs is what a request with show_nsfw off must never see.
func (e *nsfwEnv) hiddenIDs() []string {
	return []string{e.folderHidden, e.bookHidden}
}

func newNSFWEnv(t *testing.T) *nsfwEnv {
	t.Helper()
	return newNSFWEnvWithContent(t, map[string]string{})
}

// newNSFWEnvWithContent builds the fixture, giving each book the content named
// for its role. A book with no entry gets an empty source, which is what leaves
// its character count unknown and so makes it a candidate for the content-stats
// sweep.
func newNSFWEnvWithContent(t *testing.T, content map[string]string) *nsfwEnv {
	t.Helper()

	libRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(libRoot, "shelf.json"), []byte(nsfwShelfConfig), 0644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}

	app := fingerprintTestApp(t, libRoot)
	shelfData, ok := app.ShelfManager().GetShelf("default_shelf")
	if !ok {
		t.Fatal("default_shelf is missing")
	}

	env := &nsfwEnv{app: app, libRoot: libRoot, shelfData: shelfData}
	env.visible = makeBookIn(t, shelfData, libRoot, nil, "Visible", content["visible"]).ID()
	env.classic = makeBookIn(t, shelfData, libRoot, folderPathOf("Fiction/Classics"), "Classic", content["classic"]).ID()
	env.folderHidden = makeBookIn(t, shelfData, libRoot, folderPathOf(nsfwMarkedFolder), "FolderHidden", content["folderHidden"]).ID()

	bookHidden := makeBookIn(t, shelfData, libRoot, folderPathOf(nsfwFlaggedFolder), "BookHidden", content["bookHidden"])
	meta := *bookHidden.GetMeta()
	meta.NSFW = true
	if err := bookHidden.SetMeta(&meta); err != nil {
		t.Fatalf("marking %s nsfw: %v", bookHidden.ID(), err)
	}
	env.bookHidden = bookHidden.ID()

	empty := folderPathOf(nsfwEmptyFolder)
	if err := shelfData.NewFolder(empty[:len(empty)-1], empty[len(empty)-1]); err != nil {
		t.Fatalf("NewFolder(%s): %v", nsfwEmptyFolder, err)
	}

	// The nsfw flag was written straight to book.json, so rebuild the book cache
	// from disk rather than trusting the entry that predates it.
	if _, err := shelfData.RescanUnthrottled(); err != nil {
		t.Fatalf("RescanUnthrottled: %v", err)
	}

	return env
}

func folderPathOf(folder string) shelf.FolderPath {
	return shelf.FolderPath(strings.Split(folder, "/"))
}

// setShowNSFW stores the setting the way a client would, through the API.
func (e *nsfwEnv) setShowNSFW(t *testing.T, show bool) {
	t.Helper()

	value := "false"
	if show {
		value = "true"
	}
	rec := e.do(t, http.MethodPost, "/api/setting/show_nsfw", strings.NewReader(value))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST show_nsfw = %d, want 204; body %s", rec.Code, rec.Body.String())
	}
}

// do issues a request through the whole handler, token attached, so these tests
// exercise the routing and the security gate as a client would.
func (e *nsfwEnv) do(t *testing.T, method, url string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, url, body)
	req.Header.Set(e.app.SecurityTokenHeader(), e.app.SecurityToken())
	rec := httptest.NewRecorder()
	e.app.Handler().ServeHTTP(rec, req)
	return rec
}

func (e *nsfwEnv) get(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, url, nil)
}

func shelfURL(parts ...string) string {
	return "/api/shelves/default_shelf/" + path.Join(parts...)
}

func bookURL(bookID string, parts ...string) string {
	return shelfURL(append([]string{"books", bookID}, parts...)...)
}

func decodeInto[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var value T
	if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	return value
}

func (e *nsfwEnv) listedBookIDs(t *testing.T) []string {
	t.Helper()

	books := decodeInto[[]Book](t, e.get(t, shelfURL("books")))
	ids := make([]string, 0, len(books))
	for _, book := range books {
		ids = append(ids, book.Meta.ID)
	}
	slices.Sort(ids)
	return ids
}

func assertIDs(t *testing.T, got []string, want ...string) {
	t.Helper()

	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("book IDs = %v, want %v", got, want)
	}
}

// The setting itself: hidden is what a server that has never been told
// otherwise does, and deleting the stored value returns to it.
func TestShowNSFWDefaultsToHiddenAndRoundTrips(t *testing.T) {
	env := newNSFWEnv(t)

	readValue := func() bool {
		return decodeInto[struct {
			Value bool `json:"value"`
		}](t, env.get(t, "/api/setting/show_nsfw")).Value
	}

	if readValue() {
		t.Fatal("show_nsfw = true with nothing stored, want the hidden default")
	}

	env.setShowNSFW(t, true)
	if !readValue() {
		t.Fatal("show_nsfw = false after storing true")
	}

	if rec := env.do(t, http.MethodDelete, "/api/setting/show_nsfw", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE show_nsfw = %d, want 204; body %s", rec.Code, rec.Body.String())
	}
	if readValue() {
		t.Fatal("show_nsfw = true after the stored value was deleted")
	}

	if rec := env.do(t, http.MethodPost, "/api/setting/show_nsfw", strings.NewReader(`"yes"`)); rec.Code != http.StatusBadRequest {
		t.Fatalf("POST show_nsfw with a non-boolean body = %d, want 400", rec.Code)
	}
}

func TestListingsExcludeNSFWBooksWhenHidden(t *testing.T) {
	env := newNSFWEnv(t)

	assertIDs(t, env.listedBookIDs(t), env.visible, env.classic)

	env.setShowNSFW(t, true)
	assertIDs(t, env.listedBookIDs(t), env.visible, env.classic, env.folderHidden, env.bookHidden)
}

// Every book here has an empty source, so they all share one MD5 and the
// duplicate endpoint groups exactly the books it can see.
func TestDuplicateBooksExcludeNSFWBooksWhenHidden(t *testing.T) {
	env := newNSFWEnv(t)

	duplicateIDs := func() []string {
		groups := decodeInto[[][]string](t, env.get(t, shelfURL("books", "duplicate")))
		ids := []string{}
		for _, group := range groups {
			ids = append(ids, group...)
		}
		slices.Sort(ids)
		return ids
	}

	assertIDs(t, duplicateIDs(), env.visible, env.classic)

	env.setShowNSFW(t, true)
	assertIDs(t, duplicateIDs(), env.visible, env.classic, env.folderHidden, env.bookHidden)
}

// distinctText is variedText with a vocabulary of its own, so the two share no
// shingle and the books built from them are never a similar pair.
func distinctText(tokens int) string {
	var b strings.Builder
	for i := range tokens {
		b.WriteString("chapter")
		b.WriteString(strconv.Itoa(i))
		b.WriteByte(' ')
	}
	return b.String()
}

// A hidden book is compared against nothing, so it cannot appear as the other
// half of a visible book's pair either.
func TestSimilarBooksExcludeNSFWBooksWhenHidden(t *testing.T) {
	shared := variedText(200)
	env := newNSFWEnvWithContent(t, map[string]string{
		"visible":      shared,
		"classic":      distinctText(200),
		"folderHidden": shared,
		"bookHidden":   shared,
	})

	books := []*shelf.Book{}
	for _, id := range []string{env.visible, env.classic, env.folderHidden, env.bookHidden} {
		book, err := env.shelfData.GetBook(id)
		if err != nil {
			t.Fatalf("GetBook(%s): %v", id, err)
		}
		books = append(books, book)
	}
	fingerprintBooks(t, env.shelfData, books...)

	pairedIDs := func() []string {
		pairs := decodeInto[[]similarPair](t, env.get(t, shelfURL("books", "similar")))
		ids := []string{}
		for _, pair := range pairs {
			ids = append(ids, pair.A, pair.B)
		}
		slices.Sort(ids)
		return slices.Compact(ids)
	}

	if got := pairedIDs(); len(got) != 0 {
		t.Errorf("paired IDs = %v, want none: the only match for Visible is hidden", got)
	}

	env.setShowNSFW(t, true)
	assertIDs(t, pairedIDs(), env.visible, env.folderHidden, env.bookHidden)
}

// The sweep is background work started by a request, so what it may touch is
// decided by that request. Its reported total is the observable half: it counts
// the books it swept, and a hidden one must not be among them.
func TestContentStatRefreshSkipsNSFWBooksWhenHidden(t *testing.T) {
	env := newNSFWEnv(t)

	sweptTotal := func() int {
		rec := env.do(t, http.MethodPost, shelfURL("content-stat-refreshes"), nil)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("POST content-stat-refreshes = %d, want 202; body %s", rec.Code, rec.Body.String())
		}
		var submitted struct {
			TaskChainID string `json:"taskchain_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &submitted); err != nil {
			t.Fatalf("decoding %q: %v", rec.Body.String(), err)
		}

		deadline := time.Now().Add(10 * time.Second)
		for {
			chain := decodeInto[TaskChain](t, env.get(t, "/api/taskchains/"+submitted.TaskChainID))
			if chain.Status == "completed" || chain.Status == "partially_completed" || chain.Status == "failed" {
				if chain.Status != "completed" {
					t.Fatalf("chain finished %q, want completed", chain.Status)
				}
				result, err := json.Marshal(chain.Tasks[0].Result)
				if err != nil {
					t.Fatalf("re-encoding the task result: %v", err)
				}
				var stats struct {
					Total int `json:"total"`
				}
				if err := json.Unmarshal(result, &stats); err != nil {
					t.Fatalf("decoding %q: %v", result, err)
				}
				return stats.Total
			}
			if time.Now().After(deadline) {
				t.Fatalf("chain %s did not finish, last status %q", submitted.TaskChainID, chain.Status)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	if got := sweptTotal(); got != 2 {
		t.Errorf("swept %d books, want the 2 this request can see", got)
	}

	env.setShowNSFW(t, true)
	if got := sweptTotal(); got != 4 {
		t.Errorf("swept %d books with show_nsfw on, want all 4", got)
	}
}

// Every route that names one book answers as though it were not there. The
// response is compared against the one an ID that was never issued gets, so a
// difference in status, code or message is a difference a caller could measure.
func TestNamedNSFWBookRoutesAnswerNotFoundWhenHidden(t *testing.T) {
	env := newNSFWEnv(t)

	// A cover to fetch, so a 404 on the cover route is the visibility filter
	// rather than a book that simply has no cover. Seeding one onto a marked
	// book needs the setting on, which is itself the reverse case: the upload
	// route is only reachable while the book is visible.
	env.setShowNSFW(t, true)
	for _, id := range []string{env.visible, env.folderHidden, env.bookHidden} {
		req := httptest.NewRequest(http.MethodPut, bookURL(id, "cover"), bytes.NewReader([]byte("not really a png")))
		req.Header.Set("Content-Type", "image/png")
		req.Header.Set(env.app.SecurityTokenHeader(), env.app.SecurityToken())
		rec := httptest.NewRecorder()
		env.app.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("PUT cover for %s = %d, want 204; body %s", id, rec.Code, rec.Body.String())
		}
	}

	env.setShowNSFW(t, false)

	sourceID := func(bookID string) string {
		book, err := env.shelfData.GetBook(bookID)
		if err != nil {
			t.Fatalf("GetBook(%s): %v", bookID, err)
		}
		return book.CurrentSource()
	}

	routes := []struct {
		name   string
		method string
		parts  []string
		body   string
	}{
		{name: "get book", method: http.MethodGet},
		{name: "patch book", method: http.MethodPatch, body: `{"title":"renamed"}`},
		{name: "delete book", method: http.MethodDelete},
		{name: "get cover", method: http.MethodGet, parts: []string{"cover"}},
		{name: "delete cover", method: http.MethodDelete, parts: []string{"cover"}},
		{name: "get content", method: http.MethodGet, parts: []string{"content"}},
		{name: "list sources", method: http.MethodGet, parts: []string{"sources"}},
		{name: "copy book", method: http.MethodPost, parts: []string{"copies"}},
	}

	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			// "no_such_book" was never issued, so its response is what a book
			// that is not on this shelf looks like.
			wantRec := env.do(t, tc.method, bookURL("no_such_book", tc.parts...), body)
			if wantRec.Code != http.StatusNotFound {
				t.Fatalf("unknown book %s = %d, want 404; body %s", tc.name, wantRec.Code, wantRec.Body.String())
			}

			for _, hidden := range env.hiddenIDs() {
				if tc.body != "" {
					body = strings.NewReader(tc.body)
				}
				rec := env.do(t, tc.method, bookURL(hidden, tc.parts...), body)
				assertSameRefusal(t, rec, wantRec)
			}
		})
	}

	// The source routes take a real source ID, so they are checked separately:
	// the point is that the book is not found before its source is even looked
	// for.
	t.Run("source content", func(t *testing.T) {
		wantRec := env.do(t, http.MethodGet, bookURL("no_such_book", "sources", sourceID(env.visible), "content"), nil)
		if wantRec.Code != http.StatusNotFound {
			t.Fatalf("unknown book = %d, want 404; body %s", wantRec.Code, wantRec.Body.String())
		}
		for _, hidden := range env.hiddenIDs() {
			rec := env.do(t, http.MethodGet, bookURL(hidden, "sources", sourceID(hidden), "content"), nil)
			assertSameRefusal(t, rec, wantRec)
		}
	})

	// The same routes on the same books once the setting is on: nothing above
	// was a book that had gone missing.
	env.setShowNSFW(t, true)
	for _, hidden := range env.hiddenIDs() {
		if rec := env.get(t, bookURL(hidden)); rec.Code != http.StatusOK {
			t.Errorf("GET %s with show_nsfw on = %d, want 200; body %s", hidden, rec.Code, rec.Body.String())
		}
		if rec := env.get(t, bookURL(hidden, "cover")); rec.Code != http.StatusOK {
			t.Errorf("GET cover of %s with show_nsfw on = %d, want 200", hidden, rec.Code)
		}
		if rec := env.get(t, bookURL(hidden, "content")); rec.Code != http.StatusOK {
			t.Errorf("GET content of %s with show_nsfw on = %d, want 200", hidden, rec.Code)
		}
	}
}

// assertSameRefusal compares everything a caller can read off the two responses
// except the incident ID, which is per-request by design.
func assertSameRefusal(t *testing.T, got, want *httptest.ResponseRecorder) {
	t.Helper()

	if got.Code != want.Code {
		t.Errorf("status = %d, want %d (the unknown-book answer)", got.Code, want.Code)
	}

	type envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	var gotBody, wantBody envelope
	if err := json.Unmarshal(got.Body.Bytes(), &gotBody); err != nil {
		t.Fatalf("decoding %q: %v", got.Body.String(), err)
	}
	if err := json.Unmarshal(want.Body.Bytes(), &wantBody); err != nil {
		t.Fatalf("decoding %q: %v", want.Body.String(), err)
	}
	if gotBody != wantBody {
		t.Errorf("error body = %+v, want %+v (the unknown-book answer)", gotBody, wantBody)
	}
}

func TestFolderTreeDropsFoldersLeftWithoutVisibleBooks(t *testing.T) {
	env := newNSFWEnv(t)

	folders := func() []string {
		listed := decodeInto[[]shelf.FolderPath](t, env.get(t, shelfURL("folders")))
		paths := make([]string, 0, len(listed))
		for _, folder := range listed {
			paths = append(paths, folder.String())
		}
		slices.Sort(paths)
		return paths
	}

	got := folders()
	for _, want := range []string{"Fiction", "Fiction/Classics", nsfwEmptyFolder} {
		if !slices.Contains(got, want) {
			t.Errorf("folders = %v, want it to contain %q", got, want)
		}
	}
	// Marked, and holding only a marked book, respectively.
	for _, unwanted := range []string{nsfwMarkedFolder, nsfwFlaggedFolder} {
		if slices.Contains(got, unwanted) {
			t.Errorf("folders = %v, want %q dropped", got, unwanted)
		}
	}

	env.setShowNSFW(t, true)
	got = folders()
	for _, want := range []string{"Fiction", "Fiction/Classics", nsfwEmptyFolder, nsfwMarkedFolder, nsfwFlaggedFolder} {
		if !slices.Contains(got, want) {
			t.Errorf("folders with show_nsfw on = %v, want it to contain %q", got, want)
		}
	}
}

// The export is a mirror of the shelf, not of this machine's display
// preferences: the Android client reading it applies the mark itself, and a
// cache pruned to one server's setting would silently become that client's
// whole library.
func TestBookCacheExportIsCompleteWhateverShowNSFWSays(t *testing.T) {
	env := newNSFWEnv(t)

	exportedIDs := func() []string {
		rec := env.do(t, http.MethodPost, shelfURL("book-cache-exports"), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST book-cache-exports = %d, want 200; body %s", rec.Code, rec.Body.String())
		}

		matches, err := filepath.Glob(filepath.Join(env.libRoot, "app", "book-cache-*.json"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("glob for the exported cache = %v (%v), want exactly one file", matches, err)
		}
		bs, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatalf("read %s: %v", matches[0], err)
		}

		var file struct {
			Books map[string]struct {
				NSFW bool `json:"nsfw"`
			} `json:"books"`
		}
		if err := json.Unmarshal(bs, &file); err != nil {
			t.Fatalf("decoding %s: %v", matches[0], err)
		}

		ids := make([]string, 0, len(file.Books))
		for id, entry := range file.Books {
			ids = append(ids, id)
			wantMark := id == env.folderHidden || id == env.bookHidden
			if entry.NSFW != wantMark {
				t.Errorf("exported nsfw for %s = %v, want %v", id, entry.NSFW, wantMark)
			}
		}
		slices.Sort(ids)
		return ids
	}

	assertIDs(t, exportedIDs(), env.visible, env.classic, env.folderHidden, env.bookHidden)

	env.setShowNSFW(t, true)
	assertIDs(t, exportedIDs(), env.visible, env.classic, env.folderHidden, env.bookHidden)
}
