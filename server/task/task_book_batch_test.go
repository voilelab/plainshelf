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

func TestBookBatchTaskNoOpStillCompletes(t *testing.T) {
	shelfDir := path.Join(t.TempDir(), "shelf")
	newShelf, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: shelfDir})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	defer newShelf.Close()
	newShelf.WaitReady(t.Context())

	book, err := newShelf.NewBook(shelf.FolderPath{"target"}, "test")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	logger, err := logutil.NewLogger(&logutil.LogConf{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	task := newBookBatchTask("default_shelf", newShelf, logger,
		BookBatchOperationMove, []string{book.ID()}, shelf.FolderPath{"target"}, nil)

	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.Status() != taskutil.StatusCompleted || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want completed at 100%%", task.Status(), task.Percentage())
	}
	result := task.Result().(BookBatchResult)
	if len(result.SucceededIDs) != 1 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want one no-op success", result)
	}
}

func TestBookBatchTaskAllFailuresStillProcessesEveryBook(t *testing.T) {
	shelfDir := path.Join(t.TempDir(), "shelf")
	newShelf, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: shelfDir})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	defer newShelf.Close()
	newShelf.WaitReady(t.Context())

	_, err = newShelf.NewBook(shelf.FolderPath{}, "test")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	logger, err := logutil.NewLogger(&logutil.LogConf{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	task := newBookBatchTask("default_shelf", newShelf, logger,
		BookBatchOperationTrash, []string{"missing-a", "missing-b"}, shelf.FolderPath{}, nil)

	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run returned an item error: %v", err)
	}
	if task.Status() != taskutil.StatusFailed || task.Percentage() != 100 {
		t.Fatalf("status = %v at %v%%, want failed at 100%%", task.Status(), task.Percentage())
	}
	result := task.Result().(BookBatchResult)
	if len(result.SucceededIDs) != 0 || len(result.Failures) != 2 {
		t.Fatalf("result = %+v, want two failures", result)
	}
	for _, failure := range result.Failures {
		if failure.Code != "not_found" {
			t.Errorf("failure code = %q, want not_found", failure.Code)
		}
	}
}

func TestBookBatchTaskCancelledBeforeStart(t *testing.T) {
	shelfDir := path.Join(t.TempDir(), "shelf")
	newShelf, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: shelfDir})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	defer newShelf.Close()
	newShelf.WaitReady(t.Context())

	book, err := newShelf.NewBook(shelf.FolderPath{}, "test")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	logger, err := logutil.NewLogger(&logutil.LogConf{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	task := newBookBatchTask("default_shelf", newShelf, logger,
		BookBatchOperationTrash, []string{book.ID()}, shelf.FolderPath{}, nil)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = task.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusFailed || task.Percentage() != 0 {
		t.Fatalf("status = %v at %v%%, want failed at 0%%", task.Status(), task.Percentage())
	}
	result := task.Result().(BookBatchResult)
	if len(result.SucceededIDs) != 0 || len(result.Failures) != 0 {
		t.Fatalf("cancelled task recorded item results: %+v", result)
	}

	if _, err := newShelf.GetBook(book.ID()); err != nil {
		t.Fatalf("cancelled task changed the book: %v", err)
	}
}

func TestBookBatchTaskResultSnapshotsAreIndependent(t *testing.T) {
	shelfDir := path.Join(t.TempDir(), "shelf")
	newShelf, err := shelf.NewShelf(&shelf.ShelfConf{LibRoot: shelfDir})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	defer newShelf.Close()
	newShelf.WaitReady(t.Context())

	book, err := newShelf.NewBook(shelf.FolderPath{}, "test")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	logger, err := logutil.NewLogger(&logutil.LogConf{})
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	task := newBookBatchTask("default_shelf", newShelf, logger,
		BookBatchOperationMove, []string{book.ID()}, shelf.FolderPath{"target"}, nil)

	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	first := task.Result().(BookBatchResult)
	first.SucceededIDs[0] = "mutated"
	first.Failures = append(first.Failures, bookBatchFailure{BookID: "fake", Code: "move_failed"})
	second := task.Result().(BookBatchResult)
	if len(second.SucceededIDs) != 1 || second.SucceededIDs[0] != book.ID() || len(second.Failures) != 0 {
		t.Fatalf("Result shared mutable storage: %+v", second)
	}
}
