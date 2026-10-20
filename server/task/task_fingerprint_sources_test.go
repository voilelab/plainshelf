package task

import (
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func newFingerprintTestShelf(t *testing.T, libRoot string, readOnly bool) *shelf.Shelf {
	t.Helper()

	testShelf, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: libRoot, LockMode: "none", ReadOnly: readOnly})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	t.Cleanup(func() { _ = testShelf.Close() })
	testShelf.WaitReady(t.Context())

	return testShelf
}

func newTaskTestLogger(t *testing.T) *logutil.Logger {
	t.Helper()

	logger, err := logutil.NewLogger(&logutil.LogConf{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	return logger
}

func sourceFilePaths(t *testing.T, libRoot string, book *shelf.Book) []string {
	t.Helper()

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}

	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		paths = append(paths, path.Join(libRoot, source.FolderPath(), shelf.SourceFile))
	}
	return paths
}

func addFingerprintBook(t *testing.T, testShelf *shelf.Shelf, libRoot, title, content string) *shelf.Book {
	t.Helper()

	book, err := testShelf.NewBookWith(nil, title, func(book *shelf.Book) error {
		source, err := book.NewSource(strings.NewReader(content))
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		t.Fatalf("NewBookWith(%q): %v", title, err)
	}

	old := time.Now().Add(-time.Hour)
	for _, sourcePath := range sourceFilePaths(t, libRoot, book) {
		if err := os.Chtimes(sourcePath, old, old); err != nil {
			t.Fatalf("backdating %s: %v", sourcePath, err)
		}
	}

	return book
}

func addSource(t *testing.T, libRoot string, book *shelf.Book, content string) {
	t.Helper()

	if _, err := book.NewSource(strings.NewReader(content)); err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	old := time.Now().Add(-time.Hour)
	for _, sourcePath := range sourceFilePaths(t, libRoot, book) {
		if err := os.Chtimes(sourcePath, old, old); err != nil {
			t.Fatalf("backdating %s: %v", sourcePath, err)
		}
	}
}

func TestFingerprintSourcesTaskCoversEverySourceOfABook(t *testing.T) {
	libRoot := t.TempDir()
	testShelf := newFingerprintTestShelf(t, libRoot, false)

	book := addFingerprintBook(t, testShelf, libRoot, "Dune", "the spice must flow")
	addSource(t, libRoot, book, "the spice must flow, revised")

	task, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := fingerprintResult(t, task)
	if got.Books != 1 || got.Sources != 2 || got.Fingerprinted != 2 || got.Built != 2 {
		t.Fatalf("result = %+v, want both sources of the one book fingerprinted", got)
	}

	second, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := fingerprintResult(t, second); got.StatHits != 2 || got.Built != 0 {
		t.Errorf("second run = %+v, want both sources answered from the index", got)
	}
}

func runFingerprintSources(t *testing.T, testShelf *shelf.Shelf) (*fingerprintSourcesTask, error) {
	t.Helper()

	task := newFingerprintSourcesTask("default_shelf", testShelf, false, newTaskTestLogger(t), nil)
	return task, task.Run(t.Context())
}

func runForceFingerprintSources(t *testing.T, testShelf *shelf.Shelf) (*fingerprintSourcesTask, error) {
	t.Helper()

	task := newFingerprintSourcesTask("default_shelf", testShelf, true, newTaskTestLogger(t), nil)
	return task, task.Run(t.Context())
}

func fingerprintResult(t *testing.T, task *fingerprintSourcesTask) FingerprintSourcesResult {
	t.Helper()

	result, ok := task.Result().(FingerprintSourcesResult)
	if !ok {
		t.Fatalf("Result() = %T, want FingerprintSourcesResult", task.Result())
	}
	return result
}

func TestFingerprintSourcesTaskOnlyPaysForWhatChanged(t *testing.T) {
	libRoot := t.TempDir()
	testShelf := newFingerprintTestShelf(t, libRoot, false)

	addFingerprintBook(t, testShelf, libRoot, "Dune", "the spice must flow")
	addFingerprintBook(t, testShelf, libRoot, "Solaris", "the ocean thinks in shapes")

	first, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if first.Status() != taskutil.StatusCompleted || first.Percentage() != 100 {
		t.Fatalf("first run: status %v at %v%%, want completed at 100%%", first.Status(), first.Percentage())
	}

	got := fingerprintResult(t, first)
	if got.Books != 2 || got.Sources != 2 || got.Fingerprinted != 2 || got.Built != 2 {
		t.Fatalf("first run result = %+v, want two books, two sources, two builds", got)
	}
	if len(got.Failures) != 0 {
		t.Fatalf("first run reported failures: %+v", got.Failures)
	}

	second, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	got = fingerprintResult(t, second)
	if got.StatHits != 2 || got.Built != 0 || got.ContentHits != 0 {
		t.Errorf("second run result = %+v, want two stat hits and no reads at all", got)
	}
	if got.Fingerprinted != 2 {
		t.Errorf("second run fingerprinted %d sources, want 2", got.Fingerprinted)
	}

	addFingerprintBook(t, testShelf, libRoot, "Ubik", "the coin-operated door")

	third, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("third run: %v", err)
	}

	got = fingerprintResult(t, third)
	if got.Books != 3 || got.Built != 1 || got.StatHits != 2 {
		t.Errorf("third run result = %+v, want only the new book fingerprinted", got)
	}
}

func TestFingerprintSourcesTaskForceRebuildsCachedSources(t *testing.T) {
	libRoot := t.TempDir()
	testShelf := newFingerprintTestShelf(t, libRoot, false)

	addFingerprintBook(t, testShelf, libRoot, "Dune", "the spice must flow")
	addFingerprintBook(t, testShelf, libRoot, "Solaris", "the ocean thinks in shapes")

	if _, err := runFingerprintSources(t, testShelf); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := fingerprintResult(t, second); got.StatHits != 2 || got.Built != 0 || got.ContentHits != 0 {
		t.Fatalf("second run = %+v, want two stat hits and no reads", got)
	}

	forced, err := runForceFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("force run: %v", err)
	}
	if forced.Status() != taskutil.StatusCompleted || forced.Percentage() != 100 {
		t.Fatalf("force run: status %v at %v%%, want completed at 100%%", forced.Status(), forced.Percentage())
	}

	if got := fingerprintResult(t, forced); got.Fingerprinted != 2 || got.Built != 2 || got.StatHits != 0 || got.ContentHits != 0 {
		t.Errorf("force run = %+v, want both sources rebuilt with no cache hits", got)
	}
}

func TestFingerprintSourcesTaskSeesAnEditedSource(t *testing.T) {
	libRoot := t.TempDir()
	testShelf := newFingerprintTestShelf(t, libRoot, false)

	book := addFingerprintBook(t, testShelf, libRoot, "Dune", "the spice must flow")

	if _, err := runFingerprintSources(t, testShelf); err != nil {
		t.Fatalf("first run: %v", err)
	}

	sourcePath := sourceFilePaths(t, libRoot, book)[0]
	if err := os.WriteFile(sourcePath, []byte("edited from outside"), 0o644); err != nil {
		t.Fatalf("editing source.txt: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(sourcePath, old, old); err != nil {
		t.Fatalf("backdating %s: %v", sourcePath, err)
	}

	second, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if got := fingerprintResult(t, second); got.Built != 1 || got.StatHits != 0 {
		t.Errorf("result = %+v, want the edited source rebuilt", got)
	}
}

func TestFingerprintSourcesTaskReportsOneUnreadableSource(t *testing.T) {
	libRoot := t.TempDir()
	testShelf := newFingerprintTestShelf(t, libRoot, false)

	broken := addFingerprintBook(t, testShelf, libRoot, "Truncated", "gone by the time it is read")
	addFingerprintBook(t, testShelf, libRoot, "Intact", "still here")

	if err := os.Remove(sourceFilePaths(t, libRoot, broken)[0]); err != nil {
		t.Fatalf("removing source.txt: %v", err)
	}

	task, err := runFingerprintSources(t, testShelf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusPartiallyCompleted || task.Percentage() != 100 {
		t.Fatalf("status %v at %v%%, want partially completed at 100%%", task.Status(), task.Percentage())
	}

	got := fingerprintResult(t, task)
	if got.Fingerprinted != 1 || got.Built != 1 {
		t.Errorf("result = %+v, want the intact book still fingerprinted", got)
	}
	if len(got.Failures) != 1 {
		t.Fatalf("failures = %+v, want exactly one", got.Failures)
	}
	if got.Failures[0].BookID != broken.ID() || got.Failures[0].SourceID == "" {
		t.Errorf("failure = %+v, want the broken book and the source that failed", got.Failures[0])
	}
}

func TestFingerprintSourcesTaskFailsOnAReadOnlyShelf(t *testing.T) {
	libRoot := t.TempDir()

	writable := newFingerprintTestShelf(t, libRoot, false)
	addFingerprintBook(t, writable, libRoot, "Dune", "the spice must flow")
	if err := writable.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	readOnly := newFingerprintTestShelf(t, libRoot, true)

	task, err := runFingerprintSources(t, readOnly)
	if err == nil {
		t.Fatal("Run on a read-only shelf returned no error")
	}
	if task.Status() != taskutil.StatusFailed {
		t.Errorf("status = %v, want failed", task.Status())
	}
}

func TestFingerprintAlgoIsComplete(t *testing.T) {
	algo := FingerprintAlgo()
	if algo.Normalize == "" || algo.Shingle == "" || algo.Hash == "" || algo.K < 1 {
		t.Fatalf("FingerprintAlgo() = %+v, want every field set", algo)
	}
}

func TestBuildFingerprintDescribesTheContent(t *testing.T) {
	spaced, err := BuildFingerprint([]byte("the spice   must\n\nflow"))
	if err != nil {
		t.Fatalf("BuildFingerprint: %v", err)
	}
	if spaced.Sketch == "" || spaced.NormMD5 == "" {
		t.Fatalf("fingerprint = %+v, want a sketch and a normalized hash", spaced)
	}
	if spaced.NormChars != len("thespicemustflow") {
		t.Errorf("norm_chars = %d, want the length of the normalized text", spaced.NormChars)
	}

	rewrapped, err := BuildFingerprint([]byte("the spice\nmust flow"))
	if err != nil {
		t.Fatalf("BuildFingerprint: %v", err)
	}
	if rewrapped != spaced {
		t.Errorf("rewrapped fingerprint = %+v, want the same as %+v", rewrapped, spaced)
	}

	empty, err := BuildFingerprint(nil)
	if err != nil {
		t.Fatalf("BuildFingerprint(nil): %v", err)
	}
	if empty.Sketch == "" {
		t.Error("an empty source produced no sketch")
	}
}
