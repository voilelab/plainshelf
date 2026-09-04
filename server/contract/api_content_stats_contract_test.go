package contract_test

import (
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

// clearSourceCharCount rewrites a book's current source meta.json with a zero
// character count, reproducing the on-disk state of a book whose count was
// never computed. Once the shelf has been walked again the API omits
// char_count for it, which is exactly what the maintenance page reports as an
// unknown count.
func clearSourceCharCount(t *testing.T, env *apitest.Env, bookID string) {
	t.Helper()

	bookDir := filepath.Dir(env.BookMetaPath(t, bookID))
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

func charCountByBookID(t *testing.T, env *apitest.Env) map[string]int {
	t.Helper()

	counts := make(map[string]int)
	for _, book := range apitest.GetJSON[[]server.Book](t, env, apitest.BooksURL()+"?include=char_count") {
		if book.Meta == nil {
			continue
		}
		counts[book.Meta.ID] = book.CharCount
	}
	return counts
}

func refreshContentStats(t *testing.T, env *apitest.Env, wantStatus int) apitest.TaskChainSubmitResponse {
	t.Helper()
	return apitest.SubmitTaskChain(t, env, apitest.ContentStatsURL(), nil, wantStatus)
}

func TestAPIRefreshContentStatsContract(t *testing.T) {
	env := apitest.New(t)

	stale := apitest.ImportTextBook(t, env, "Stale Stats", "", "stale.txt", "alpha body")
	fresh := apitest.ImportTextBook(t, env, "Fresh Stats", "", "fresh.txt", "beta body")

	want := charCountByBookID(t, env)[stale.Meta.ID]
	if want == 0 {
		t.Fatalf("imported book should start with a positive char_count")
	}

	clearSourceCharCount(t, env, stale.Meta.ID)

	// The listing reports counts held in the book cache, so an edit made
	// straight on disk reaches it through a walk of the shelf - the rescan a
	// user runs after changing a shelf from outside PlainShelf.
	apitest.AssertStatus(t, env.Post(apitest.ScansURL(), nil), http.StatusOK)

	if got := charCountByBookID(t, env)[stale.Meta.ID]; got != 0 {
		t.Fatalf("char_count after clearing = %d, want 0", got)
	}

	accepted := refreshContentStats(t, env, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if chain.Name != task.RefreshContentStatsTaskName {
		t.Errorf("name = %q, want %q", chain.Name, task.RefreshContentStatsTaskName)
	}

	result := apitest.TaskResult[task.RefreshContentStatsResult](t, chain)
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
	env := apitest.New(t)
	_ = apitest.ImportTextBook(t, env, "Already Counted", "", "counted.txt", "alpha body")

	accepted := refreshContentStats(t, env, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "completed" {
		t.Errorf("chain status = %q, want completed", chain.Status)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if result := apitest.TaskResult[task.RefreshContentStatsResult](t, chain); result.Total != 0 || result.Refreshed != 0 {
		t.Errorf("result = %+v, want an empty sweep", result)
	}
}

// A second request while a sweep is still in flight must point the client at
// the existing chain instead of queueing a redundant one.
func TestAPIRefreshContentStatsConflictReportsRunningChainContract(t *testing.T) {
	env := apitest.New(t)

	first := apitest.AssertDuplicateChainConflict(t, env, func(wantStatus int) apitest.TaskChainSubmitResponse {
		return refreshContentStats(t, env, wantStatus)
	})

	// Once the sweep has finished, a fresh request is accepted again.
	if next := refreshContentStats(t, env, http.StatusAccepted); next.TaskChainID == first.TaskChainID {
		t.Errorf("expected a new chain after the previous one finished")
	}
}
