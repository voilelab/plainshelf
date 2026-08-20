package contract_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

func contentStatsURL() string {
	return shelfURL("content-stat-refreshes")
}

// clearSourceCharCount rewrites a book's current source meta.json with a zero
// character count, reproducing the on-disk state of a book whose count was
// never computed. Once the shelf has been walked again the API omits
// char_count for it, which is exactly what the maintenance page reports as an
// unknown count.
func clearSourceCharCount(t *testing.T, env *apiTestEnv, bookID string) {
	t.Helper()

	bookDir := filepath.Dir(env.bookMetaPath(t, bookID))
	sourcesDir := filepath.Join(bookDir, shelf.SourcesFolder)
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		t.Fatalf("read sources dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(sourcesDir, entry.Name(), shelf.SourceMetaFile)
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read source meta: %v", err)
		}
		var meta map[string]any
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatalf("decode source meta: %v", err)
		}
		delete(meta, "char_count")
		updated, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("encode source meta: %v", err)
		}
		if err := os.WriteFile(metaPath, updated, 0o644); err != nil {
			t.Fatalf("write source meta: %v", err)
		}
	}
}

func charCountByBookID(t *testing.T, env *apiTestEnv) map[string]int {
	t.Helper()

	counts := make(map[string]int)
	for _, book := range getJSON[[]server.Book](t, env, booksURL()+"?include=char_count") {
		if book.Meta == nil {
			continue
		}
		counts[book.Meta.ID] = book.CharCount
	}
	return counts
}

func refreshContentStats(t *testing.T, env *apiTestEnv, wantStatus int) taskChainSubmitResponse {
	t.Helper()
	return submitTaskChain(t, env, contentStatsURL(), nil, wantStatus)
}

func TestAPIRefreshContentStatsContract(t *testing.T) {
	env := newAPITestEnv(t)

	stale := importTextBook(t, env, "Stale Stats", "", "stale.txt", "alpha body")
	fresh := importTextBook(t, env, "Fresh Stats", "", "fresh.txt", "beta body")

	want := charCountByBookID(t, env)[stale.Meta.ID]
	if want == 0 {
		t.Fatalf("imported book should start with a positive char_count")
	}

	clearSourceCharCount(t, env, stale.Meta.ID)

	// The listing reports counts held in the book cache, so an edit made
	// straight on disk reaches it through a walk of the shelf - the rescan a
	// user runs after changing a shelf from outside PlainShelf.
	assertStatus(t, env.post(scansURL(), nil), http.StatusOK)

	if got := charCountByBookID(t, env)[stale.Meta.ID]; got != 0 {
		t.Fatalf("char_count after clearing = %d, want 0", got)
	}

	accepted := refreshContentStats(t, env, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if chain.Name != task.RefreshContentStatsTaskName {
		t.Errorf("name = %q, want %q", chain.Name, task.RefreshContentStatsTaskName)
	}

	result := taskResult[task.RefreshContentStatsResult](t, chain)
	if result.Total != 1 || result.Refreshed != 1 || len(result.Failures) != 0 {
		t.Errorf("result = %+v, want total 1, refreshed 1, no failures", result)
	}

	counts := charCountByBookID(t, env)
	if counts[stale.Meta.ID] != want {
		t.Errorf("char_count after refresh = %d, want %d", counts[stale.Meta.ID], want)
	}
	if counts[fresh.Meta.ID] == 0 {
		t.Errorf("a book with a known char_count must keep it")
	}
}

// Nothing to recompute still completes, so the UI can report "up to date"
// instead of an error.
func TestAPIRefreshContentStatsWithNothingToDoContract(t *testing.T) {
	env := newAPITestEnv(t)
	_ = importTextBook(t, env, "Already Counted", "", "counted.txt", "alpha body")

	accepted := refreshContentStats(t, env, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "completed" {
		t.Errorf("chain status = %q, want completed", chain.Status)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if result := taskResult[task.RefreshContentStatsResult](t, chain); result.Total != 0 || result.Refreshed != 0 {
		t.Errorf("result = %+v, want an empty sweep", result)
	}
}

// A second request while a sweep is still in flight must point the client at
// the existing chain instead of queueing a redundant one.
func TestAPIRefreshContentStatsConflictReportsRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t)

	release := blockWorker(t, env)

	first := refreshContentStats(t, env, http.StatusAccepted)
	conflict := refreshContentStats(t, env, http.StatusConflict)

	if conflict.TaskChainID != first.TaskChainID {
		t.Errorf("conflict taskchain_id = %q, want the running chain %q", conflict.TaskChainID, first.TaskChainID)
	}

	release()
	waitForTaskChain(t, env, first.TaskChainID)

	next := refreshContentStats(t, env, http.StatusAccepted)
	if next.TaskChainID == first.TaskChainID {
		t.Errorf("expected a new chain after the previous one finished")
	}
}

func TestAPIRefreshContentStatsRejectsUnknownShelfContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.post(shelfIDURL("missing_shelf", "content-stat-refreshes"), nil)
	assertStatus(t, rec, http.StatusNotFound)
}

// The endpoint rewrites meta.json, so it must sit inside the local_token boundary
// and be refused in read-only mode.
func TestAPIRefreshContentStatsIsGatedContract(t *testing.T) {
	env := newAPITestEnv(t)

	assertMutationGated(t, env, http.MethodPost, contentStatsURL(), nil)
}
