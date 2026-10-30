package task

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func newTransferTestShelf(t *testing.T, name string) *shelf.Shelf {
	t.Helper()
	s, _ := newTransferTestShelfAt(t, name)
	return s
}

func newTransferTestShelfAt(t *testing.T, name string) (*shelf.Shelf, string) {
	t.Helper()
	libRoot := path.Join(t.TempDir(), name)
	s, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: libRoot})
	if err != nil {
		t.Fatalf("NewShelf %s: %v", name, err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady %s: %v", name, err)
	}
	return s, libRoot
}

func folderMetaPath(libRoot string, folder ...string) string {
	return filepath.Join(append([]string{libRoot, "books"}, append(folder, shelf.FolderMetaFile)...)...)
}

func writeFolderMeta(t *testing.T, libRoot, content string, folder ...string) {
	t.Helper()
	if err := os.WriteFile(folderMetaPath(libRoot, folder...), []byte(content), 0o600); err != nil {
		t.Fatalf("write folder.json in %v: %v", folder, err)
	}
}

func readFolderMeta(t *testing.T, libRoot string, folder ...string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(folderMetaPath(libRoot, folder...))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatalf("read folder.json in %v: %v", folder, err)
	}
	return string(data), true
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

func folderTransferPlan(t *testing.T, source *shelf.Shelf, sourceFolder shelf.FolderPath) ([]FolderTransferBook, []shelf.FolderPath) {
	t.Helper()

	allFolders, err := source.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	var subfolders []shelf.FolderPath
	for _, l := range allFolders {
		if l.HasPrefix(sourceFolder) {
			subfolders = append(subfolders, l)
		}
	}

	listings, err := source.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	var books []FolderTransferBook
	for _, listing := range listings {
		if listing.Folders.HasPrefix(sourceFolder) {
			books = append(books, FolderTransferBook{ID: listing.Book.ID(), SourceFolder: listing.Folders})
		}
	}
	return books, subfolders
}

func bookIDsUnderFolder(t *testing.T, s *shelf.Shelf, folder shelf.FolderPath) []string {
	t.Helper()
	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	ids := make([]string, 0, len(listings))
	for _, listing := range listings {
		if listing.Folders.Equal(folder) {
			ids = append(ids, listing.Book.ID())
		}
	}
	return ids
}

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

	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}

	result := task.Result().(FolderTransferResult)
	if result.Total != 2 || len(result.SucceededIDs) != 2 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want two copied and no failures", result)
	}

	for _, tc := range []struct {
		original string
		folder   shelf.FolderPath
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

	targetFolders, err := target.GetAllFolders()
	if err != nil {
		t.Fatalf("target GetAllFolders: %v", err)
	}
	if !hasFolder(targetFolders, shelf.FolderPath{"imported", "empty"}) {
		t.Errorf("empty sub-folder was not reproduced on the target: %v", targetFolders)
	}

	for _, id := range []string{topID, deepID} {
		if _, err := source.GetBook(id); err != nil {
			t.Errorf("source lost %q after a copy: %v", id, err)
		}
	}
}

func TestFolderTransferTaskMoveNested(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	topID := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
	deepID := seedTransferBook(t, source, shelf.FolderPath{"fiction", "sci-fi"}, "Deep")

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)

	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}

	if got := bookIDsUnderFolder(t, target, shelf.FolderPath{"archive"}); len(got) != 1 || got[0] != topID {
		t.Errorf("target [archive] = %v, want [%s]", got, topID)
	}
	if got := bookIDsUnderFolder(t, target, shelf.FolderPath{"archive", "sci-fi"}); len(got) != 1 || got[0] != deepID {
		t.Errorf("target [archive sci-fi] = %v, want [%s]", got, deepID)
	}

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

func TestFolderTransferTaskCarriesFolderMeta(t *testing.T) {
	carried := map[string]string{
		"fiction":         `{"schema_version":9,"future":{"x":[1]}}`,
		"fiction/sci-fi":  "{\n  \"icon\": \"rocket\",\n  \"extra\": true\n}",
		"fiction/empty":   `{"icon":`,
		"fiction/no-meta": "",
	}
	for _, operation := range []string{BookTransferOperationCopy, BookTransferOperationMove} {
		t.Run(operation, func(t *testing.T) {
			source, sourceRoot := newTransferTestShelfAt(t, "source")
			target, targetRoot := newTransferTestShelfAt(t, "target")

			seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
			seedTransferBook(t, source, shelf.FolderPath{"fiction", "sci-fi"}, "Deep")
			for _, name := range []string{"empty", "no-meta"} {
				if err := source.NewFolder(shelf.FolderPath{"fiction"}, name); err != nil {
					t.Fatalf("NewFolder: %v", err)
				}
			}
			for folder, content := range carried {
				if content != "" {
					writeFolderMeta(t, sourceRoot, content, strings.Split(folder, "/")...)
				}
			}

			books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
			task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
				operation, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)
			if err := task.Run(t.Context()); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if task.Status() != taskutil.StatusCompleted {
				t.Fatalf("status = %v, result = %+v, want completed", task.Status(), task.Result())
			}

			for folder, content := range carried {
				targetFolder := append([]string{"archive"}, strings.Split(folder, "/")[1:]...)
				got, ok := readFolderMeta(t, targetRoot, targetFolder...)
				if content == "" && ok {
					t.Errorf("target %v gained a folder.json: %q", targetFolder, got)
				}
				if content != "" && got != content {
					t.Errorf("target %v folder.json = %q (present %v), want %q", targetFolder, got, ok, content)
				}
			}

			listed, err := target.ListFolderMeta()
			if err != nil {
				t.Fatalf("ListFolderMeta: %v", err)
			}
			if listed["archive/sci-fi"].Icon != "rocket" || !listed["archive/empty"].ReadOnly {
				t.Errorf("target listing = %+v", listed)
			}

			_, err = os.Stat(filepath.Join(sourceRoot, "books", "fiction"))
			if operation == BookTransferOperationMove && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("move left the source folder in place: %v", err)
			}
			if operation == BookTransferOperationCopy {
				for folder, content := range carried {
					if got, _ := readFolderMeta(t, sourceRoot, strings.Split(folder, "/")...); got != content {
						t.Errorf("copy changed source %s folder.json to %q", folder, got)
					}
				}
			}
		})
	}
}

func TestFolderMoveKeepsUncarriedMeta(t *testing.T) {
	source, sourceRoot := newTransferTestShelfAt(t, "source")
	target, targetRoot := newTransferTestShelfAt(t, "target")

	seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Top")
	if err := source.NewFolder(shelf.FolderPath{"fiction"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	writeFolderMeta(t, sourceRoot, `{"icon":"moon"}`, "fiction")
	writeFolderMeta(t, sourceRoot, `{"icon":"rocket"}`, "fiction", "empty")

	if err := target.NewFolder(shelf.FolderPath{}, "archive"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if err := target.NewFolder(shelf.FolderPath{"archive"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	writeFolderMeta(t, targetRoot, `{"icon":"star"}`, "archive", "empty")

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)
	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusPartiallyCompleted {
		t.Fatalf("status = %v, want partially_completed", task.Status())
	}
	if result := task.Result().(FolderTransferResult); result.FolderFailures != 1 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want one folder failure and no book failure", result)
	}

	if got, _ := readFolderMeta(t, targetRoot, "archive", "empty"); got != `{"icon":"star"}` {
		t.Errorf("target folder.json overwritten with %q", got)
	}
	if got, _ := readFolderMeta(t, targetRoot, "archive"); got != `{"icon":"moon"}` {
		t.Errorf("target archive folder.json = %q, want the carried one", got)
	}
	if got, _ := readFolderMeta(t, sourceRoot, "fiction", "empty"); got != `{"icon":"rocket"}` {
		t.Errorf("source lost uncarried folder.json, now %q", got)
	}
	if got, _ := readFolderMeta(t, sourceRoot, "fiction"); got != `{"icon":"moon"}` {
		t.Errorf("source parent folder.json = %q, want it kept with its child", got)
	}
}

func TestFolderTransferCancelBeforeStart(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Only")

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)

	ctx, cancel := context.WithCancel(t.Context())
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

	targetFolders, err := target.GetAllFolders()
	if err != nil {
		t.Fatalf("target GetAllFolders: %v", err)
	}
	if hasFolder(targetFolders, shelf.FolderPath{"archive"}) {
		t.Errorf("cancelled-before-start transfer created a destination folder: %v", targetFolders)
	}
}

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

func TestFolderTransferTaskCancelledMidMove(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")

	idA := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Alpha")
	idB := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Bravo")
	first, second := idA, idB
	if second < first {
		first, second = second, first
	}

	if err := source.SetFolderIcon(shelf.FolderPath{"fiction"}, "moon", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	if err := source.NewFolder(shelf.FolderPath{"fiction"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	books, subfolders := folderTransferPlan(t, source, shelf.FolderPath{"fiction"})
	task := newFolderTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, shelf.FolderPath{"fiction"}, shelf.FolderPath{"archive"}, books, subfolders)

	ctx := &cancelWhenDone{Context: t.Context(), task: task, after: 1}
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

	if meta, err := source.ReadFolderMeta(shelf.FolderPath{"fiction"}); err != nil || meta.Icon != "moon" {
		t.Errorf("source folder.json after the cancel = %+v, %v", meta, err)
	}
	if folders, err := source.GetAllFolders(); err != nil || !hasFolder(folders, shelf.FolderPath{"fiction", "empty"}) {
		t.Errorf("a cancelled move pruned an emptied source folder: %v, %v", folders, err)
	}
}

func TestFolderTransferCreateFailure(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	if err := source.NewFolder(shelf.FolderPath{}, "fiction"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

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

	if err := task.Run(t.Context()); err != nil {
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
