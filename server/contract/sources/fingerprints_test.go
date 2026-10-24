package sources_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/server/contract/apitest"

	"github.com/voilelab/plainshelf/server/task"
	"github.com/voilelab/plainshelf/shelf"
)

func backdateSourcesForFingerprintCache(t *testing.T, env *apitest.Env, bookID string) {
	t.Helper()

	sourcesDir := filepath.Join(filepath.Dir(env.BookMetaPath(t, bookID)), shelf.SourcesFolder)
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		t.Fatalf("read sources dir: %v", err)
	}

	old := time.Now().Add(-time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(sourcesDir, entry.Name(), shelf.SourceFile)
		if err := os.Chtimes(sourcePath, old, old); err != nil {
			t.Fatalf("backdating %s: %v", sourcePath, err)
		}
	}
}

func fingerprintSources(t *testing.T, env *apitest.Env, wantStatus int) apitest.TaskChainSubmitResponse {
	t.Helper()
	return apitest.SubmitTaskChain(t, env, apitest.SourceFingerprintsURL(), nil, wantStatus)
}

func forceFingerprintSources(t *testing.T, env *apitest.Env, wantStatus int) apitest.TaskChainSubmitResponse {
	t.Helper()
	return apitest.SubmitTaskChain(t, env, apitest.SourceFingerprintsURL()+"?force=true", nil, wantStatus)
}

func runFingerprintSources(t *testing.T, env *apitest.Env) task.FingerprintSourcesResult {
	t.Helper()
	return awaitFingerprintSources(t, env, fingerprintSources(t, env, http.StatusAccepted))
}

func runForceFingerprintSources(t *testing.T, env *apitest.Env) task.FingerprintSourcesResult {
	t.Helper()
	return awaitFingerprintSources(t, env, forceFingerprintSources(t, env, http.StatusAccepted))
}

func awaitFingerprintSources(t *testing.T, env *apitest.Env, accepted apitest.TaskChainSubmitResponse) task.FingerprintSourcesResult {
	t.Helper()

	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "completed" {
		t.Fatalf("chain status = %q, want completed: %+v", chain.Status, chain)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}
	if chain.Name != task.FingerprintSourcesTaskName {
		t.Errorf("name = %q, want %q", chain.Name, task.FingerprintSourcesTaskName)
	}

	return apitest.TaskResult[task.FingerprintSourcesResult](t, chain)
}

func TestAPIFingerprintSourcesContract(t *testing.T) {
	env := apitest.New(t)

	dune := apitest.ImportTextBook(t, env, "Dune", "", "dune.txt", "the spice must flow")
	solaris := apitest.ImportTextBook(t, env, "Solaris", "", "solaris.txt", "the ocean thinks in shapes")
	backdateSourcesForFingerprintCache(t, env, dune.Meta.ID)
	backdateSourcesForFingerprintCache(t, env, solaris.Meta.ID)

	first := runFingerprintSources(t, env)
	if first.Books != 2 || first.Sources != 2 || first.Fingerprinted != 2 || first.Built != 2 {
		t.Fatalf("first run = %+v, want two books fingerprinted from scratch", first)
	}
	if len(first.Failures) != 0 {
		t.Fatalf("first run reported failures: %+v", first.Failures)
	}

	second := runFingerprintSources(t, env)
	if second.StatHits != 2 || second.Built != 0 || second.ContentHits != 0 {
		t.Errorf("second run = %+v, want two stat hits and no reads", second)
	}

	ubik := apitest.ImportTextBook(t, env, "Ubik", "", "ubik.txt", "the coin-operated door")
	backdateSourcesForFingerprintCache(t, env, ubik.Meta.ID)

	third := runFingerprintSources(t, env)
	if third.Books != 3 || third.Built != 1 || third.StatHits != 2 {
		t.Errorf("third run = %+v, want only the new book fingerprinted", third)
	}
}

func TestAPIFingerprintForceRebuilds(t *testing.T) {
	env := apitest.New(t)

	dune := apitest.ImportTextBook(t, env, "Dune", "", "dune.txt", "the spice must flow")
	solaris := apitest.ImportTextBook(t, env, "Solaris", "", "solaris.txt", "the ocean thinks in shapes")
	backdateSourcesForFingerprintCache(t, env, dune.Meta.ID)
	backdateSourcesForFingerprintCache(t, env, solaris.Meta.ID)

	if first := runFingerprintSources(t, env); first.Built != 2 {
		t.Fatalf("first run = %+v, want two builds", first)
	}

	if second := runFingerprintSources(t, env); second.StatHits != 2 || second.Built != 0 {
		t.Fatalf("second run = %+v, want two stat hits and no builds", second)
	}

	forced := runForceFingerprintSources(t, env)
	if forced.Built != 2 || forced.StatHits != 0 || forced.ContentHits != 0 {
		t.Errorf("force run = %+v, want both sources rebuilt with no cache hits", forced)
	}
}

func TestAPIFingerprintRejectsBadForce(t *testing.T) {
	env := apitest.New(t)

	rec := env.Post(apitest.SourceFingerprintsURL()+"?force=maybe", nil)
	apitest.AssertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIFingerprintNoop(t *testing.T) {
	env := apitest.New(t)

	if result := runFingerprintSources(t, env); result.Books != 0 || result.Sources != 0 {
		t.Errorf("result = %+v, want an empty sweep", result)
	}
}

func TestAPIFingerprintReportsFailures(t *testing.T) {
	env := apitest.New(t)

	broken := apitest.ImportTextBook(t, env, "Truncated", "", "truncated.txt", "gone by the time it is read")
	intact := apitest.ImportTextBook(t, env, "Intact", "", "intact.txt", "still here")
	backdateSourcesForFingerprintCache(t, env, intact.Meta.ID)

	sourcesDir := filepath.Join(filepath.Dir(env.BookMetaPath(t, broken.Meta.ID)), shelf.SourcesFolder)
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		t.Fatalf("read sources dir: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(sourcesDir, entry.Name(), shelf.SourceFile)); err != nil {
			t.Fatalf("removing source.txt: %v", err)
		}
	}

	accepted := fingerprintSources(t, env, http.StatusAccepted)
	chain := apitest.WaitForTaskChain(t, env, accepted.TaskChainID)

	if chain.Status != "partially_completed" {
		t.Fatalf("chain status = %q, want partially_completed: %+v", chain.Status, chain)
	}
	if chain.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", chain.Percentage)
	}

	result := apitest.TaskResult[task.FingerprintSourcesResult](t, chain)
	if result.Fingerprinted != 1 {
		t.Errorf("result = %+v, want the intact book still fingerprinted", result)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %+v, want exactly one", result.Failures)
	}
	if result.Failures[0].BookID != broken.Meta.ID || result.Failures[0].SourceID == "" {
		t.Errorf("failure = %+v, want the broken book and the source that failed", result.Failures[0])
	}
}

func TestAPIFingerprintRunningConflict(t *testing.T) {
	env := apitest.New(t)

	first := apitest.AssertDuplicateChainConflict(t, env, func(wantStatus int) apitest.TaskChainSubmitResponse {
		return fingerprintSources(t, env, wantStatus)
	})

	if next := fingerprintSources(t, env, http.StatusAccepted); next.TaskChainID == first.TaskChainID {
		t.Errorf("expected a new chain after the previous one finished")
	}
}
