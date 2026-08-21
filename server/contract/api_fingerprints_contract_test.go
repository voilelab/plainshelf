package contract_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

func fingerprintRefreshURL() string {
	return shelfURL("fingerprint-refreshes")
}

func refreshFingerprints(t *testing.T, env *apiTestEnv, wantStatus int) taskChainSubmitResponse {
	t.Helper()
	return submitTaskChain(t, env, fingerprintRefreshURL(), nil, wantStatus)
}

// runFingerprintSweep submits a sweep, waits for it, and returns its result.
func runFingerprintSweep(t *testing.T, env *apiTestEnv) task.FingerprintSourcesResult {
	t.Helper()

	accepted := refreshFingerprints(t, env, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Name != task.FingerprintSourcesTaskName {
		t.Errorf("name = %q, want %q", chain.Name, task.FingerprintSourcesTaskName)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	return taskResult[task.FingerprintSourcesResult](t, chain)
}

// fingerprintCache reads the file the sweep writes, which is where its whole
// product lives.
func fingerprintCache(t *testing.T, env *apiTestEnv) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(env.libRoot, "app", shelf.FingerprintCacheFileName))
	if err != nil {
		t.Fatalf("read fingerprint cache: %v", err)
	}

	var cache map[string]any
	if err := json.Unmarshal(raw, &cache); err != nil {
		t.Fatalf("decode fingerprint cache: %v", err)
	}
	return cache
}

// The first sweep fingerprints every source and runs Progress to 100. The second
// one over an unchanged shelf reads no content at all, which the result reports
// as every source reused.
func TestAPIFingerprintSourcesContract(t *testing.T) {
	env := newAPITestEnv(t)

	_ = importTextBook(t, env, "First Book", "", "first.txt", strings.Repeat("alpha body text. ", 40))
	_ = importTextBook(t, env, "Second Book", "", "second.txt", strings.Repeat("beta body text. ", 40))

	first := runFingerprintSweep(t, env)
	if first.Total != 2 || first.Computed != 2 {
		t.Fatalf("first sweep = %+v, want 2 sources computed", first)
	}
	if len(first.Failures) != 0 {
		t.Errorf("first sweep failures = %+v, want none", first.Failures)
	}

	cache := fingerprintCache(t, env)
	if index, ok := cache["index"].(map[string]any); !ok || len(index) != 2 {
		t.Errorf("cache index = %v, want one entry per source", cache["index"])
	}
	if entries, ok := cache["entries"].(map[string]any); !ok || len(entries) != 2 {
		t.Errorf("cache entries = %v, want one fingerprint per distinct text", cache["entries"])
	}
	if _, ok := cache["algo"].(map[string]any); !ok {
		t.Errorf("cache is missing the algo block that invalidates it: %v", cache)
	}

	second := runFingerprintSweep(t, env)
	if second.Total != 2 || second.Reused != 2 || second.Computed != 0 {
		t.Errorf("second sweep = %+v, want every source answered from the cache", second)
	}
}

// Adding a book after a sweep costs exactly that book, not the whole shelf.
func TestAPIFingerprintSourcesIsIncrementalContract(t *testing.T) {
	env := newAPITestEnv(t)

	_ = importTextBook(t, env, "Existing Book", "", "existing.txt", strings.Repeat("alpha body text. ", 40))
	if first := runFingerprintSweep(t, env); first.Computed != 1 {
		t.Fatalf("first sweep = %+v, want the one book computed", first)
	}

	_ = importTextBook(t, env, "Added Book", "", "added.txt", strings.Repeat("gamma body text. ", 40))

	second := runFingerprintSweep(t, env)
	if second.Total != 2 || second.Computed != 1 || second.Reused != 1 {
		t.Errorf("sweep after adding a book = %+v, want one computed and one reused", second)
	}
}

// A source that cannot be read is reported in failures and does not stop the
// rest of the sweep. The chain still reaches 100%, because the percentage counts
// sources processed rather than sources fingerprinted.
func TestAPIFingerprintSourcesReportsFailuresContract(t *testing.T) {
	env := newAPITestEnv(t)

	damaged := importTextBook(t, env, "Damaged Book", "", "damaged.txt", strings.Repeat("alpha body text. ", 40))
	_ = importTextBook(t, env, "Healthy Book", "", "healthy.txt", strings.Repeat("beta body text. ", 40))

	sourceID := env.currentSourceID(t, damaged.Meta.ID)
	bookDir := filepath.Dir(env.bookMetaPath(t, damaged.Meta.ID))
	sourcePath := filepath.Join(bookDir, shelf.SourcesFolder, sourceID, shelf.SourceFile)
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("remove source content: %v", err)
	}

	accepted := refreshFingerprints(t, env, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "partially_completed" {
		t.Errorf("chain status = %q, want partially_completed", chain.Status)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}

	result := taskResult[task.FingerprintSourcesResult](t, chain)
	if result.Total != 2 || result.Computed != 1 {
		t.Errorf("result = %+v, want the healthy book fingerprinted", result)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %+v, want the damaged source alone", result.Failures)
	}
	failure := result.Failures[0]
	if failure.BookID != damaged.Meta.ID || failure.SourceID != sourceID || failure.Code == "" {
		t.Errorf("failure = %+v, want the damaged source with a code", failure)
	}
}

// Nothing to fingerprint still completes, so the UI can report "up to date"
// instead of an error.
func TestAPIFingerprintSourcesWithNoBooksContract(t *testing.T) {
	env := newAPITestEnv(t)

	accepted := refreshFingerprints(t, env, http.StatusAccepted)
	chain := waitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "completed" {
		t.Errorf("chain status = %q, want completed", chain.Status)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if result := taskResult[task.FingerprintSourcesResult](t, chain); result.Total != 0 {
		t.Errorf("result = %+v, want an empty sweep", result)
	}
}

// A second request while a sweep is still in flight must point the client at the
// existing chain instead of queueing a redundant one.
func TestAPIFingerprintSourcesConflictReportsRunningChainContract(t *testing.T) {
	env := newAPITestEnv(t)

	release := blockWorker(t, env)

	first := refreshFingerprints(t, env, http.StatusAccepted)
	conflict := refreshFingerprints(t, env, http.StatusConflict)

	if conflict.TaskChainID != first.TaskChainID {
		t.Errorf("conflict taskchain_id = %q, want the running chain %q", conflict.TaskChainID, first.TaskChainID)
	}

	release()
	waitForTaskChain(t, env, first.TaskChainID)
}

func TestAPIFingerprintSourcesRejectsUnknownShelfContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.post(shelfIDURL("missing_shelf", "fingerprint-refreshes"), nil)
	assertStatus(t, rec, http.StatusNotFound)
}

// The sweep writes app/fingerprint-cache.json, so it sits inside the local_token
// boundary and is refused on a read-only shelf.
func TestAPIFingerprintSourcesIsGatedContract(t *testing.T) {
	env := newAPITestEnv(t)

	assertMutationGated(t, env, http.MethodPost, fingerprintRefreshURL(), nil)
}

// A read-only shelf is refused at submission with a readable answer rather than
// a queued chain that would silently keep nothing.
func TestAPIFingerprintSourcesOnReadOnlyShelfContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlyShelf())

	rec := env.post(fingerprintRefreshURL(), nil)
	assertStatus(t, rec, http.StatusConflict)

	if strings.Contains(rec.Body.String(), "taskchain_id") {
		t.Errorf("body = %s, want a refusal rather than a queued chain", rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) == "" {
		t.Error("a refused sweep must say why, not answer with an empty body")
	}
}

// The sweep must not hold the shelf lock for its duration: the lock is a flock
// with a finite timeout, and holding it across the whole sweep would block
// every read for as long as the sweep takes.
//
// The guarantee is structural — the sweep takes the lock once per shelf listing
// and reads content through the shelf's read handle, which takes none — so this
// is a smoke check against an outright block rather than a measurement. It fails
// loudly if a regression ever serializes reads behind a sweep.
func TestAPIFingerprintSourcesKeepsReadsServedContract(t *testing.T) {
	env := newAPITestEnv(t)

	titles := []string{"One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight"}
	for _, title := range titles {
		// Distinct bodies, so every book costs a real read and a real sketch
		// rather than deduplicating against the first one.
		body := title + " " + strings.Repeat("body text for the fingerprint sweep. ", 200)
		_ = importTextBook(t, env, title, "", strings.ToLower(title)+".txt", body)
	}

	accepted := refreshFingerprints(t, env, http.StatusAccepted)

	duringSweep := 0
	for {
		chain := getJSON[server.TaskChain](t, env, taskChainURL(accepted.TaskChainID))
		running := chain.Status != "completed" && chain.Status != "partially_completed" && chain.Status != "failed"

		rec := env.get(booksURL())
		assertStatus(t, rec, http.StatusOK)
		if running {
			duringSweep++
			continue
		}
		break
	}

	t.Logf("served %d book listings while the sweep was in flight", duringSweep)

	if result := taskResult[task.FingerprintSourcesResult](t,
		getJSON[server.TaskChain](t, env, taskChainURL(accepted.TaskChainID))); result.Computed != len(titles) {
		t.Errorf("result = %+v, want all %d books fingerprinted", result, len(titles))
	}
}
