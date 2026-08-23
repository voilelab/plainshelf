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

func seedTransferBook(t *testing.T, s *shelf.Shelf, layer shelf.FolderPath, title string) string {
	t.Helper()
	book, err := s.NewBook(layer, title)
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

// layerTransferPlan mirrors what the endpoint resolves before scheduling: the
// layers under sourceLayer and the books beneath it. Building it from the same
// exported reads the handler uses keeps the task tests honest about its input.
func layerTransferPlan(t *testing.T, source *shelf.Shelf, sourceLayer shelf.FolderPath) ([]LayerTransferBook, []shelf.FolderPath) {
	t.Helper()

	allLayers, err := source.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllLayers: %v", err)
	}
	var sublayers []shelf.FolderPath
	for _, l := range allLayers {
		if layerHasPrefix(l, sourceLayer) {
			sublayers = append(sublayers, l)
		}
	}

	listings, err := source.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	var books []LayerTransferBook
	for _, listing := range listings {
		if layerHasPrefix(listing.Folders, sourceLayer) {
			books = append(books, LayerTransferBook{ID: listing.Book.ID(), SourceLayer: listing.Folders})
		}
	}
	return books, sublayers
}

func bookIDsUnderLayer(t *testing.T, s *shelf.Shelf, layer shelf.FolderPath) []string {
	t.Helper()
	books, err := s.GetBooksByFolder(layer)
	if err != nil {
		t.Fatalf("GetBooksByLayer %v: %v", layer, err)
	}
	ids := make([]string, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID())
	}
	return ids
}

// A copy transfer reproduces the whole subtree - nested layers and an empty
// sub-folder - on the target with fresh IDs and leaves the source untouched.
func TestLayerTransferTaskCopyNested(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	topID := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
	deepID := seedTransferBook(t, source, shelf.FolderPath{"fiction", "sci-fi"}, "Deep")
	if err := source.NewFolder(shelf.FolderPath{"fiction"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	books, sublayers := layerTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newLayerTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationCopy, shelf.FolderPath{"fiction"}, shelf.FolderPath{"imported"}, books, sublayers)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}

	result := task.Result().(LayerTransferResult)
	if result.Total != 2 || len(result.SucceededIDs) != 2 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want two copied and no failures", result)
	}

	// Every book lands under its remapped layer with a fresh ID.
	for _, tc := range []struct {
		original string
		layer    shelf.FolderPath
	}{
		{topID, shelf.FolderPath{"imported"}},
		{deepID, shelf.FolderPath{"imported", "sci-fi"}},
	} {
		ids := bookIDsUnderLayer(t, target, tc.layer)
		if len(ids) != 1 {
			t.Fatalf("target layer %v holds %v, want one book", tc.layer, ids)
		}
		if ids[0] == tc.original {
			t.Errorf("copy under %v reused the original ID %q", tc.layer, tc.original)
		}
	}

	// The empty sub-folder is reproduced on the target.
	targetLayers, err := target.GetAllFolders()
	if err != nil {
		t.Fatalf("target GetAllLayers: %v", err)
	}
	if !hasLayer(targetLayers, shelf.FolderPath{"imported", "empty"}) {
		t.Errorf("empty sub-layer was not reproduced on the target: %v", targetLayers)
	}

	// The source keeps both books under their original IDs.
	for _, id := range []string{topID, deepID} {
		if _, err := source.GetBook(id); err != nil {
			t.Errorf("source lost %q after a copy: %v", id, err)
		}
	}
}

// A move transfer keeps every book's ID, publishes the subtree on the target, and
// removes the emptied source layer once every book has moved.
func TestLayerTransferTaskMoveNested(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	topID := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
	deepID := seedTransferBook(t, source, shelf.FolderPath{"fiction", "sci-fi"}, "Deep")

	books, sublayers := layerTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newLayerTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, sublayers)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}

	// The target lists both books under their original IDs at the remapped layers.
	if got := bookIDsUnderLayer(t, target, shelf.FolderPath{"archive"}); len(got) != 1 || got[0] != topID {
		t.Errorf("target [archive] = %v, want [%s]", got, topID)
	}
	if got := bookIDsUnderLayer(t, target, shelf.FolderPath{"archive", "sci-fi"}); len(got) != 1 || got[0] != deepID {
		t.Errorf("target [archive sci-fi] = %v, want [%s]", got, deepID)
	}

	// The source no longer lists either book, and the emptied subtree is pruned.
	for _, id := range []string{topID, deepID} {
		if _, err := source.GetBook(id); !errors.Is(err, shelf.ErrBookNotFound) {
			t.Errorf("source still lists moved book %q, err = %v", id, err)
		}
	}
	sourceLayers, err := source.GetAllFolders()
	if err != nil {
		t.Fatalf("source GetAllLayers: %v", err)
	}
	if hasLayer(sourceLayers, shelf.FolderPath{"fiction"}) || hasLayer(sourceLayers, shelf.FolderPath{"fiction", "sci-fi"}) {
		t.Errorf("source layer subtree survived the move: %v", sourceLayers)
	}
}

// A cancel arriving before the first book leaves both shelves untouched and the
// task fails at 0%, recording no per-book results.
func TestLayerTransferTaskCancelledBeforeStart(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Only")

	books, sublayers := layerTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newLayerTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, sublayers)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := task.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusFailed || task.Percentage() != 0 {
		t.Fatalf("status = %v at %v%%, want failed at 0%%", task.Status(), task.Percentage())
	}
	result := task.Result().(LayerTransferResult)
	if len(result.SucceededIDs) != 0 || len(result.Failures) != 0 {
		t.Fatalf("cancelled task recorded item results: %+v", result)
	}

	// Nothing landed on the target - not even the destination layer.
	targetLayers, err := target.GetAllFolders()
	if err != nil {
		t.Fatalf("target GetAllLayers: %v", err)
	}
	if hasLayer(targetLayers, shelf.FolderPath{"archive"}) {
		t.Errorf("cancelled-before-start transfer created a destination layer: %v", targetLayers)
	}
}

// cancelWhenDone reports cancellation as soon as the task has published `after`
// books, so a mid-run cancel is deterministic: the task probes ctx.Err() before
// each book, sees no error until `after` have landed, then stops. It reads the
// task's own progress rather than a timer, so there is no timing race.
type cancelWhenDone struct {
	context.Context
	task  *layerTransferTask
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
func TestLayerTransferTaskCancelledMidMove(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	// Two books at the top of the layer; the transfer visits them sorted by ID.
	idA := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Alpha")
	idB := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Bravo")
	first, second := idA, idB
	if second < first {
		first, second = second, first
	}

	books, sublayers := layerTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newLayerTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, sublayers)

	ctx := &cancelWhenDone{Context: context.Background(), task: task, after: 1}
	if err := task.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusPartiallyCompleted {
		t.Fatalf("status = %v, want partially_completed", task.Status())
	}

	result := task.Result().(LayerTransferResult)
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
// an empty-layer transfer onto a read-only target creates nothing, so the task
// settles failed and records the layer failure rather than reporting completed.
func TestLayerTransferTaskLayerCreateFailureSettlesFailed(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	if err := source.NewFolder(shelf.FolderPath{}, "fiction"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	// Lay the target down writable, then reopen it read-only so NewLayer fails.
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

	books, sublayers := layerTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newLayerTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationCopy, shelf.FolderPath{"fiction"}, shelf.FolderPath{"imported"}, books, sublayers)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusFailed {
		t.Fatalf("status = %v, want failed when the destination layer cannot be created", task.Status())
	}
	if result := task.Result().(LayerTransferResult); result.LayerFailures == 0 {
		t.Fatalf("result = %+v, want a recorded layer failure", result)
	}
}

func hasLayer(layers []shelf.FolderPath, want shelf.FolderPath) bool {
	for _, l := range layers {
		if l.Equal(want) {
			return true
		}
	}
	return false
}
