package task

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const BookBatchTaskName = "book_batch"

const (
	BookBatchOperationMove  = "move"
	BookBatchOperationTrash = "trash"
)

type bookBatchFailure struct {
	BookID string `json:"book_id"`
	Code   string `json:"code"`
}

type BookBatchResult struct {
	Operation    string             `json:"operation"`
	Total        int                `json:"total"`
	SucceededIDs []string           `json:"succeeded_ids"`
	Failures     []bookBatchFailure `json:"failures"`
}

type bookBatchTask struct {
	shelfID      string
	shelf        *shelf.Shelf
	logger       *logutil.Logger
	operation    string
	bookIDs      []string
	targetFolder shelf.FolderPath

	progress taskutil.Progress
	mu       sync.Mutex
	result   BookBatchResult
}

func newBookBatchTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger, operation string, bookIDs []string, targetFolder shelf.FolderPath) *bookBatchTask {
	return &bookBatchTask{
		shelfID:      shelfID,
		shelf:        s,
		logger:       logger,
		operation:    operation,
		bookIDs:      slices.Clone(bookIDs),
		targetFolder: slices.Clone(targetFolder),
		result: BookBatchResult{
			Operation:    operation,
			Total:        len(bookIDs),
			SucceededIDs: []string{},
			Failures:     []bookBatchFailure{},
		},
	}
}

func (t *bookBatchTask) Name() string {
	return BookBatchTaskName
}

func (t *bookBatchTask) Title() string {
	if t.operation == BookBatchOperationMove {
		return "Move books"
	}
	return "Move books to trash"
}

func (t *bookBatchTask) Description() string {
	t.mu.Lock()
	succeeded, failed := len(t.result.SucceededIDs), len(t.result.Failures)
	t.mu.Unlock()

	_, total := t.progress.Counts()
	description := "moved " + strconv.Itoa(succeeded) + " of " + strconv.Itoa(total) + " books to trash"
	if t.operation == BookBatchOperationMove {
		description = "moved " + strconv.Itoa(succeeded) + " of " + strconv.Itoa(total) + " books"
	}
	if failed > 0 {
		description += ", " + strconv.Itoa(failed) + " failed"
	}
	return description
}

func (t *bookBatchTask) interruptedStatus() taskutil.Status {
	t.mu.Lock()
	processed := len(t.result.SucceededIDs) + len(t.result.Failures)
	t.mu.Unlock()
	if processed == 0 {
		return taskutil.StatusFailed
	}
	return taskutil.StatusPartiallyCompleted
}

func (t *bookBatchTask) Percentage() float64 {
	return t.progress.Percentage()
}

func (t *bookBatchTask) Status() taskutil.Status {
	return t.progress.Status()
}

func (t *bookBatchTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return BookBatchResult{
		Operation:    t.result.Operation,
		Total:        t.result.Total,
		SucceededIDs: append(make([]string, 0, len(t.result.SucceededIDs)), t.result.SucceededIDs...),
		Failures:     append(make([]bookBatchFailure, 0, len(t.result.Failures)), t.result.Failures...),
	}
}

func (t *bookBatchTask) recordSuccess(bookID string) {
	t.mu.Lock()
	t.result.SucceededIDs = append(t.result.SucceededIDs, bookID)
	t.mu.Unlock()
}

func (t *bookBatchTask) recordFailure(bookID string, err error) {
	code := t.operation + "_failed"
	if errors.Is(err, shelf.ErrBookNotFound) {
		code = "not_found"
	} else if errors.Is(err, shelf.ErrUnsupportedBookSchemaVersion) {
		code = "unsupported_schema"
	}

	t.mu.Lock()
	t.result.Failures = append(t.result.Failures, bookBatchFailure{BookID: bookID, Code: code})
	t.mu.Unlock()
	t.logger.Error("book batch item failed",
		"shelf_id", t.shelfID, "operation", t.operation, "book_id", bookID, "error", err)
}

func (t *bookBatchTask) finishStatus() taskutil.Status {
	t.mu.Lock()
	succeeded, failed := len(t.result.SucceededIDs), len(t.result.Failures)
	t.mu.Unlock()

	switch {
	case failed == 0:
		return taskutil.StatusCompleted
	case succeeded == 0:
		return taskutil.StatusFailed
	default:
		return taskutil.StatusPartiallyCompleted
	}
}

func (t *bookBatchTask) Run(ctx context.Context) error {
	t.progress.SetTotal(len(t.bookIDs))
	t.progress.SetStatus(taskutil.StatusRunning)

	for _, bookID := range t.bookIDs {
		if err := ctx.Err(); err != nil {
			t.progress.SetStatus(t.interruptedStatus())
			return util.Errorf("%w", err)
		}

		var err error
		switch t.operation {
		case BookBatchOperationMove:
			var listing shelf.BookListing
			listing, err = t.shelf.GetBookListing(bookID)
			if err == nil && listing.Folders.Equal(t.targetFolder) {
				t.recordSuccess(bookID)
				t.progress.Advance()
				continue
			}
			if err == nil {
				_, err = t.shelf.MoveBook(bookID, t.targetFolder)
			}
		case BookBatchOperationTrash:
			err = t.shelf.DeleteBook(bookID)
		}

		if err != nil {
			t.recordFailure(bookID, err)
		} else {
			t.recordSuccess(bookID)
		}
		t.progress.Advance()
	}

	t.progress.SetStatus(t.finishStatus())
	return nil
}

func NewBookBatchChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger, operation string, bookIDs []string, targetFolder shelf.FolderPath) *taskutil.TaskChain {
	keyIDs := slices.Clone(bookIDs)
	slices.Sort(keyIDs)
	key := strings.Join([]string{BookBatchTaskName, shelfID, operation, targetFolder.String(), strings.Join(keyIDs, ",")}, ":")
	task := newBookBatchTask(shelfID, s, logger, operation, bookIDs, targetFolder)
	return &taskutil.TaskChain{
		Key:         key,
		Name:        BookBatchTaskName,
		Title:       task.Title(),
		Description: "Process a batch of books",
		Tasks:       []taskutil.Task{task},
	}
}
