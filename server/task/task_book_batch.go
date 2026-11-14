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
	BookBatchOperationMove      = "move"
	BookBatchOperationTrash     = "trash"
	BookBatchOperationSetSeries = "set_series"
)

type BookBatchSeries struct {
	Name       string
	StartIndex float64
	Clear      bool
}

func (s BookBatchSeries) forPosition(position int) *shelf.BookSeries {
	if s.Clear {
		return nil
	}
	return &shelf.BookSeries{Name: s.Name, Index: s.StartIndex + float64(position)}
}

type BookBatchSpec struct {
	Operation    string
	BookIDs      []string
	TargetFolder shelf.FolderPath
	Series       BookBatchSeries
}

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
	series       BookBatchSeries

	visible VisibleBooks

	progress taskutil.Progress
	mu       sync.Mutex
	result   BookBatchResult
}

func newBookBatchTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger, spec BookBatchSpec, visible VisibleBooks) *bookBatchTask {
	return &bookBatchTask{
		shelfID:      shelfID,
		shelf:        s,
		logger:       logger,
		operation:    spec.Operation,
		bookIDs:      slices.Clone(spec.BookIDs),
		targetFolder: slices.Clone(spec.TargetFolder),
		series:       spec.Series,
		visible:      visible,
		result: BookBatchResult{
			Operation:    spec.Operation,
			Total:        len(spec.BookIDs),
			SucceededIDs: []string{},
			Failures:     []bookBatchFailure{},
		},
	}
}

func (t *bookBatchTask) Name() string {
	return BookBatchTaskName
}

func (t *bookBatchTask) Title() string {
	switch t.operation {
	case BookBatchOperationMove:
		return "Move books"
	case BookBatchOperationSetSeries:
		return "Set book series"
	default:
		return "Move books to trash"
	}
}

func (t *bookBatchTask) Description() string {
	t.mu.Lock()
	succeeded, failed := len(t.result.SucceededIDs), len(t.result.Failures)
	t.mu.Unlock()

	_, total := t.progress.Counts()
	counts := strconv.Itoa(succeeded) + " of " + strconv.Itoa(total) + " books"
	var description string
	switch t.operation {
	case BookBatchOperationMove:
		description = "moved " + counts
	case BookBatchOperationSetSeries:
		description = "updated " + counts
	default:
		description = "moved " + counts + " to trash"
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

	for position, bookID := range t.bookIDs {
		if err := ctx.Err(); err != nil {
			t.progress.SetStatus(t.interruptedStatus())
			return util.Errorf("%w", err)
		}

		if !t.visible.Allows(bookID) {
			t.recordFailure(bookID, shelf.ErrBookNotFound)
			t.progress.Advance()
			continue
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
		case BookBatchOperationSetSeries:
			err = t.setSeries(bookID, t.series.forPosition(position))
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

func (t *bookBatchTask) setSeries(bookID string, series *shelf.BookSeries) error {
	book, err := t.shelf.GetBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}
	meta := book.GetMeta()
	meta.Series = series
	if err := book.SetMeta(meta); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func NewBookBatchChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger, spec BookBatchSpec, visible VisibleBooks) *taskutil.TaskChain {
	keyIDs := slices.Clone(spec.BookIDs)
	slices.Sort(keyIDs)
	key := strings.Join([]string{BookBatchTaskName, shelfID, spec.Operation, spec.TargetFolder.String(), strings.Join(keyIDs, ",")}, ":")
	task := newBookBatchTask(shelfID, s, logger, spec, visible)
	return &taskutil.TaskChain{
		Key:         key,
		Name:        BookBatchTaskName,
		Title:       task.Title(),
		Description: "Process a batch of books",
		Tasks:       []taskutil.Task{task},
	}
}
