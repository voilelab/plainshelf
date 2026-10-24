package task

import (
	"context"
	"errors"
	"testing"

	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

func TestBookTransferTaskRefusesOnlyBeforeItStarts(t *testing.T) {
	source := newTransferTestShelf(t, "source")
	target := newTransferTestShelf(t, "target")
	bookID := seedTransferBook(t, source, shelf.FolderPath{"fiction"}, "Only")

	task := newBookTransferTask("source", source, "target", target, newTestLogger(t),
		BookTransferOperationMove, bookID, shelf.FolderPath{"archive"})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := task.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if task.Status() != taskutil.StatusFailed {
		t.Fatalf("status = %v, want failed", task.Status())
	}
	if _, err := source.GetBook(bookID); err != nil {
		t.Errorf("source lost the book a cancelled transfer never started: %v", err)
	}
	if _, err := target.GetBook(bookID); !errors.Is(err, shelf.ErrBookNotFound) {
		t.Errorf("target GetBook = %v, want ErrBookNotFound", err)
	}
}
