package task

import (
	"context"
	"errors"
	"path"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func newTransferTestShelf(t *testing.T, name string) *shelf.Shelf {
	t.Helper()
	s, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: path.Join(t.TempDir(), name)})
	if err != nil {
		t.Fatalf("NewShelf %s: %v", name, err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady %s: %v", name, err)
	}
	return s
}

func seedTransferBook(t *testing.T, s *shelf.Shelf, folder shelf.FolderPath, title string) string {
	t.Helper()
	book, err := s.NewBook(folder, title)
	if err != nil {
		t.Fatalf("NewBook %q: %v", title, err)
	}
	return book.ID()
}

func newTestLogger(t *testing.T) *logutil.Logger {
	t.Helper()
	logger, err := logutil.NewLogger(&logutil.LogConf{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	return logger
}

// folderTransferPlan mirrors what the endpoint resolves before scheduling: the
// folders under sourceFolder and the books beneath it. Building it from the same
// exported reads the handler uses keeps the task tests honest about its input.
func folderTransferPlan(t *testing.T, source *shelf.Shelf, sourceFolder shelf.FolderPath) ([]FolderTransferBook, []shelf.FolderPath) {
	t.Helper()

	allFolders, err := source.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	var subfolders []shelf.FolderPath
	for _, l := range allFolders {
		if folderHasPrefix(l, sourceFolder) {
			subfolders = append(subfolders, l)
		}
	}

	listings, err := source.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	var books []FolderTransferBook
	for _, listing := range listings {
		if folderHasPrefix(listing.Folders, sourceFolder) {
			books = append(books, FolderTransferBook{ID: listing.Book.ID(), SourceFolder: listing.Folders})
		}
	}
	return books, subfolders
}

func bookIDsUnderFolder(t *testing.T, s *shelf.Shelf, folder shelf.FolderPath) []string {
	t.Helper()
	books, err := s.GetBooksByFolder(folder)
	if err != nil {
		t.Fatalf("GetBooksByFolder %v: %v", folder, err)
	}
	ids := make([]string, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID())
	}
	return ids
}

// A copy transfer reproduces the whole subtree - nested folders and an empty
// sub-folder - on the target with fresh IDs and leaves the source untouched.
func TestFolderTransferTaskCopyNested(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	topID := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
	deepID := seedTransferBook(t, source, shelf.FolderPath{"fiction", "sci-fi"}, "Deep")
	if err := source.NewFolder(shelf.FolderPath{"fiction"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationCopy, shelf.FolderPath{"fiction"}, shelf.FolderPath{"imported"}, books, subfolders)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}

	result := task.Result().(FolderTransferResult)
	if result.Total != 2 || len(result.SucceededIDs) != 2 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want two copied and no failures", result)
	}

	// Every book lands under its remapped folder with a fresh ID.
	for _, tc := range []struct {
		original string
		folder    shelf.FolderPath
	}{
		{topID, shelf.FolderPath{"imported"}},
		{deepID, shelf.FolderPath{"imported", "sci-fi"}},
	} {
		ids := bookIDsUnderFolder(t, target, tc.folder)
		if len(ids) != 1 {
			t.Fatalf("target folder %v holds %v, want one book", tc.folder, ids)
		}
		if ids[0] == tc.original {
			t.Errorf("copy under %v reused the original ID %q", tc.folder, tc.original)
		}
	}

	// The empty sub-folder is reproduced on the target.
	targetFolders, err := target.GetAllFolders()
	if err != nil {
		t.Fatalf("target GetAllFolders: %v", err)
	}
	if !hasFolder(targetFolders, shelf.FolderPath{"imported", "empty"}) {
		t.Errorf("empty sub-folder was not reproduced on the target: %v", targetFolders)
	}

	// The source keeps both books under their original IDs.
	for _, id := range []string{topID, deepID} {
		if _, err := source.GetBook(id); err != nil {
			t.Errorf("source lost %q after a copy: %v", id, err)
		}
	}
}

// A move transfer keeps every book's ID, publishes the subtree on the target, and
// removes the emptied source folder once every book has moved.
func TestFolderTransferTaskMoveNested(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	topID := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
	deepID := seedTransferBook(t, source, shelf.FolderPath{"fiction", "sci-fi"}, "Deep")

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}

	// The target lists both books under their original IDs at the remapped folders.
	if got := bookIDsUnderFolder(t, target, shelf.FolderPath{"archive"}); len(got) != 1 || got[0] != topID {
		t.Errorf("target [archive] = %v, want [%s]", got, topID)
	}
	if got := bookIDsUnderFolder(t, target, shelf.FolderPath{"archive", "sci-fi"}); len(got) != 1 || got[0] != deepID {
		t.Errorf("target [archive sci-fi] = %v, want [%s]", got, deepID)
	}

	// The source no longer lists either book, and the emptied subtree is pruned.
	for _, id := range []string{topID, deepID} {
		if _, err := source.GetBook(id); !errors.Is(err, shelf.ErrBookNotFound) {
			t.Errorf("source still lists moved book %q, err = %v", id, err)
		}
	}
	sourceFolders, err := source.GetAllFolders()
	if err != nil {
		t.Fatalf("source GetAllFolders: %v", err)
	}
	if hasFolder(sourceFolders, shelf.FolderPath{"fiction"}) || hasFolder(sourceFolders, shelf.FolderPath{"fiction", "sci-fi"}) {
		t.Errorf("source folder subtree survived the move: %v", sourceFolders)
	}
}

// A cancel arriving before the first book leaves both shelves untouched and the
// task fails at 0%, recording no per-book results.
func TestFolderTransferTaskCancelledBeforeStart(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Only")

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := task.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusFailed || task.Percentage() != 0 {
		t.Fatalf("status = %v at %v%%, want failed at 0%%", task.Status(), task.Percentage())
	}
	result := task.Result().(FolderTransferResult)
	if len(result.SucceededIDs) != 0 || len(result.Failures) != 0 {
		t.Fatalf("cancelled task recorded item results: %+v", result)
	}

	// Nothing landed on the target - not even the destination folder.
	targetFolders, err := target.GetAllFolders()
	if err != nil {
		t.Fatalf("target GetAllFolders: %v", err)
	}
	if hasFolder(targetFolders, shelf.FolderPath{"archive"}) {
		t.Errorf("cancelled-before-start transfer created a destination folder: %v", targetFolders)
	}
}

// cancelWhenDone reports cancellation as soon as the task has published `after`
// books, so a mid-run cancel is deterministic: the task probes ctx.Err() before
// each book, sees no error until `after` have landed, then stops. It reads the
// task's own progress rather than a timer, so there is no timing race.
type cancelWhenDone struct {
	context.Context
	task  *folderTransferTask
	after int
}

func (c *cancelWhenDone) Err() error {
	if done, _ := c.task.progress.Counts(); done >= c.after {
		return context.Canceled
	}
	return c.Context.Err()
}

// A cancel partway through a move stops before the next book, keeps the books
// already moved on the target, leaves the rest on the source, and rolls nothing
// back. The status is partially completed, never a clean completion.
func TestFolderTransferTaskCancelledMidMove(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	// Two books at the top of the folder; the transfer visits them sorted by ID.
	idA := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Alpha")
	idB := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Bravo")
	first, second := idA, idB
	if second < first {
		first, second = second, first
	}

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)

	ctx := &cancelWhenDone{Context: context.Background(), task: task, after: 1}
	if err := task.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusPartiallyCompleted {
		t.Fatalf("status = %v, want partially_completed", task.Status())
	}

	result := task.Result().(FolderTransferResult)
	if len(result.SucceededIDs) != 1 || result.SucceededIDs[0] != first {
		t.Fatalf("succeeded = %v, want just the first book %q", result.SucceededIDs, first)
	}

	// The first book moved across and stays there (no rollback); the second never
	// started and stays on the source.
	if _, err := target.GetBook(first); err != nil {
		t.Errorf("first book did not land on the target: %v", err)
	}
	if _, err := source.GetBook(first); !errors.Is(err, shelf.ErrBookNotFound) {
		t.Errorf("a cancel rolled the first book back onto the source, err = %v", err)
	}
	if _, err := source.GetBook(second); err != nil {
		t.Errorf("the second book left the source despite the cancel: %v", err)
	}
	if _, err := target.GetBook(second); !errors.Is(err, shelf.ErrBookNotFound) {
		t.Errorf("the second book was transferred after the cancel, err = %v", err)
	}
}

// A destination folder that cannot be created must not read as a clean success:
// an empty-folder transfer onto a read-only target creates nothing, so the task
// settles failed and records the folder failure rather than reporting completed.
func TestFolderTransferTaskFolderCreateFailureSettlesFailed(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	if err := source.NewFolder(shelf.FolderPath{}, "fiction"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	// Lay the target down writable, then reopen it read-only so NewFolder fails.
	targetDir := path.Join(t.TempDir(), "target")
	seed, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: targetDir})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := seed.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady seed: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}
	target, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: targetDir, ReadOnly: true})
	if err != nil {
		t.Fatalf("reopen target read-only: %v", err)
	}
	t.Cleanup(func() { target.Close() })
	if err := target.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady target: %v", err)
	}

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationCopy, shelf.FolderPath{"fiction"}, shelf.FolderPath{"imported"}, books, subfolders)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusFailed {
		t.Fatalf("status = %v, want failed when the destination folder cannot be created", task.Status())
	}
	if result := task.Result().(FolderTransferResult); result.FolderFailures == 0 {
		t.Fatalf("result = %+v, want a recorded folder failure", result)
	}
}

func hasFolder(folders []shelf.FolderPath, want shelf.FolderPath) bool {
	for _, l := range folders {
		if l.Equal(want) {
			return true
		}
	}
	return false
}
