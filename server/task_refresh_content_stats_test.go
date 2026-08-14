package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf"
)

// The sweep scans every book up front to report an accurate total, then
// recomputes them one at a time. A Source carries its whole meta.json in memory
// and writes all of it back, so a source opened during the scan and written
// during the recompute would silently undo anything stored in between.
//
// This drives the two phases with a write wedged between them — the interleaving
// a concurrent request would produce, made deterministic.
func TestRefreshContentStatsRereadsSourceBeforeWriting(t *testing.T) {
	env := newAPITestEnv(t)

	created := importTextBook(t, env, "Concurrently Edited", "", "concurrent.txt", "one\ntwo\nthree")
	bookID := created.Meta.ID
	splitURL := "/api/shelves/default_shelf/books/" + bookID + "/split_config"

	clearSourceCharCount(t, env, bookID)

	shelfData, ok := env.app.shelfManager.GetShelf("default_shelf")
	if !ok {
		t.Fatal("default_shelf not found")
	}
	task := newRefreshContentStatsTask("default_shelf", shelfData.Shelf, &env.app.Logger)

	books, err := shelfData.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}

	candidates := task.collectCandidates(books)
	if len(candidates) != 1 || candidates[0] != bookID {
		t.Fatalf("candidates = %#v, want just %q", candidates, bookID)
	}

	// The concurrent write: it lands after the scan has seen this source and
	// before the recompute reaches it.
	rec := env.do(httptest.NewRequest(http.MethodPatch, splitURL, strings.NewReader(`{"type":"line_count","line_count":42}`)))
	assertStatus(t, rec, http.StatusNoContent)

	if err := task.refreshBook(bookID); err != nil {
		t.Fatalf("refreshBook: %v", err)
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, splitURL, nil))
	assertStatus(t, rec, http.StatusOK)
	if split := decodeJSON[shelf.SplitConfig](t, rec); split.Type != shelf.SplitTypeLineCount || split.LineCount != 42 {
		t.Errorf("split config after refresh = %#v, want the concurrent line_count 42 to survive", split)
	}

	if got := charCountByBookID(t, env)[bookID]; got == 0 {
		t.Errorf("char_count after refresh = 0, want it recomputed")
	}
}
