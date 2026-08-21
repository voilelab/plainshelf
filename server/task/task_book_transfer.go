package task

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/shelf"
)

const BookTransferTaskName = "book_transfer"

const (
	BookTransferOperationCopy = "copy"
	BookTransferOperationMove = "move"
)

// BookTransferResult is what a client reads back from the chain once a
// cross-shelf transfer settles. On success BookID and Layer name where the book
// landed on the target; on failure FailureCode classifies why, the same way the
// batch task's failure codes do.
type BookTransferResult struct {
	Operation    string       `json:"operation"`
	SourceShelf  string       `json:"source_shelf"`
	TargetShelf  string       `json:"target_shelf"`
	SourceBookID string       `json:"source_book_id"`
	BookID       string       `json:"book_id,omitempty"`
	Layer        shelf.Layers `json:"layer,omitempty"`
	FailureCode  string       `json:"failure_code,omitempty"`
}

type bookTransferTask struct {
	source      *shelf.Shelf
	target      *shelf.Shelf
	logger      *logutil.Logger
	operation   string
	bookID      string
	targetLayer shelf.Layers

	progress taskutil.Progress
	mu       sync.Mutex
	result   BookTransferResult
}

func newBookTransferTask(sourceShelfID string, source *shelf.Shelf, targetShelfID string, target *shelf.Shelf, logger *logutil.Logger, operation, bookID string, targetLayer shelf.Layers) *bookTransferTask {
	return &bookTransferTask{
		source:      source,
		target:      target,
		logger:      logger,
		operation:   operation,
		bookID:      bookID,
		targetLayer: append(shelf.Layers(nil), targetLayer...),
		result: BookTransferResult{
			Operation:    operation,
			SourceShelf:  sourceShelfID,
			TargetShelf:  targetShelfID,
			SourceBookID: bookID,
		},
	}
}

func (t *bookTransferTask) Name() string { return BookTransferTaskName }

func (t *bookTransferTask) Title() string {
	if t.operation == BookTransferOperationMove {
		return "Move book to another shelf"
	}
	return "Copy book to another shelf"
}

func (t *bookTransferTask) Description() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	inProgress, done, failed := "copying a book", "copied the book", "failed to copy the book"
	if t.operation == BookTransferOperationMove {
		inProgress, done, failed = "moving a book", "moved the book", "failed to move the book"
	}
	switch {
	case t.result.FailureCode != "":
		return failed + " to " + t.result.TargetShelf
	case t.result.BookID != "":
		return done + " to " + t.result.TargetShelf
	default:
		return inProgress + " to " + t.result.TargetShelf
	}
}

func (t *bookTransferTask) Percentage() float64 { return t.progress.Percentage() }

func (t *bookTransferTask) Status() taskutil.Status { return t.progress.Status() }

func (t *bookTransferTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := t.result
	result.Layer = append(shelf.Layers(nil), t.result.Layer...)
	return result
}

// failureCode classifies a transfer error the same way the batch task does, so a
// client sees stable codes across both endpoints, plus the two failures only a
// cross-shelf transfer can raise.
func failureCode(operation string, err error) string {
	switch {
	case errors.Is(err, shelf.ErrBookNotFound):
		return "not_found"
	case errors.Is(err, shelf.ErrBookIDConflict):
		return "id_conflict"
	case errors.Is(err, shelf.ErrUnsupportedBookSchemaVersion):
		return "unsupported_schema"
	default:
		return operation + "_failed"
	}
}

func (t *bookTransferTask) Run(ctx context.Context) error {
	t.progress.SetTotal(1)
	t.progress.SetStatus(taskutil.StatusRunning)

	// The transfer itself cannot be cancelled partway - a half-copied book would
	// be published or not, never both - so only refuse before it starts.
	if err := ctx.Err(); err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		return err
	}

	var (
		book *shelf.Book
		err  error
	)
	if t.operation == BookTransferOperationMove {
		book, err = t.target.MoveBookFrom(t.source, t.bookID, t.targetLayer)
	} else {
		book, err = t.target.CopyBookFrom(t.source, t.bookID, t.targetLayer, false)
	}

	t.mu.Lock()
	if err != nil {
		t.result.FailureCode = failureCode(t.operation, err)
	} else {
		t.result.BookID = book.ID()
		t.result.Layer = book.Layers()
	}
	t.mu.Unlock()

	t.progress.Advance()

	if err != nil {
		t.logger.Error("book transfer failed",
			"operation", t.operation,
			"source_shelf", t.result.SourceShelf,
			"target_shelf", t.result.TargetShelf,
			"book_id", t.bookID,
			"error", err)
		t.progress.SetStatus(taskutil.StatusFailed)
		return nil
	}

	t.progress.SetStatus(taskutil.StatusCompleted)
	return nil
}

// NewBookTransferChain builds the one-task chain that copies or moves a single
// book from one shelf to another. The dedupe key folds in both shelves, the
// operation, the book and the destination layer so that re-submitting the same
// transfer attaches to the one already running instead of starting a second.
func NewBookTransferChain(sourceShelfID string, source *shelf.Shelf, targetShelfID string, target *shelf.Shelf, logger *logutil.Logger, operation, bookID string, targetLayer shelf.Layers) *taskutil.TaskChain {
	key := strings.Join([]string{
		BookTransferTaskName, sourceShelfID, targetShelfID, operation, bookID, targetLayer.String(),
	}, ":")
	task := newBookTransferTask(sourceShelfID, source, targetShelfID, target, logger, operation, bookID, targetLayer)
	return &taskutil.TaskChain{
		Key:         key,
		Name:        BookTransferTaskName,
		Title:       task.Title(),
		Description: "Transfer a book to another shelf",
		Tasks:       []taskutil.Task{task},
	}
}
