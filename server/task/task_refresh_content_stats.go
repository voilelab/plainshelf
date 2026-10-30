package task

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const RefreshContentStatsTaskName = "refresh_content_stats"

type refreshContentStatsFailure struct {
	BookID string `json:"book_id"`
	Code   string `json:"code"`
}

type RefreshContentStatsResult struct {
	Total     int                          `json:"total"`
	Refreshed int                          `json:"refreshed"`
	Failures  []refreshContentStatsFailure `json:"failures"`
}

type refreshContentStatsTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger

	visible VisibleBooks

	progress taskutil.Progress

	mu        sync.Mutex
	refreshed int
	failures  []refreshContentStatsFailure
}

func newRefreshContentStatsTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger, visible VisibleBooks) *refreshContentStatsTask {
	return &refreshContentStatsTask{
		shelfID:  shelfID,
		shelf:    s,
		logger:   logger,
		visible:  visible,
		failures: []refreshContentStatsFailure{},
	}
}

func (t *refreshContentStatsTask) Name() string {
	return RefreshContentStatsTaskName
}

func (t *refreshContentStatsTask) Title() string {
	return "Update content statistics"
}

func (t *refreshContentStatsTask) Description() string {
	t.mu.Lock()
	refreshed, failed := t.refreshed, len(t.failures)
	t.mu.Unlock()

	_, total := t.progress.Counts()

	desc := "updated " + strconv.Itoa(refreshed) + " of " + strconv.Itoa(total) + " books"
	if failed > 0 {
		desc += ", " + strconv.Itoa(failed) + " failed"
	}
	return desc
}

func (t *refreshContentStatsTask) Percentage() float64 {
	return t.progress.Percentage()
}

func (t *refreshContentStatsTask) Status() taskutil.Status {
	return t.progress.Status()
}

func (t *refreshContentStatsTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, total := t.progress.Counts()
	return RefreshContentStatsResult{
		Total:     total,
		Refreshed: t.refreshed,
		Failures:  append(make([]refreshContentStatsFailure, 0, len(t.failures)), t.failures...),
	}
}

func (t *refreshContentStatsTask) recordFailure(bookID string, err error) {
	code := "refresh_failed"
	switch {
	case errors.Is(err, shelf.ErrBookNotFound), errors.Is(err, shelf.ErrSourceNotFound):
		code = "not_found"
	case errors.Is(err, shelf.ErrUnsupportedBookSchemaVersion), errors.Is(err, shelf.ErrUnsupportedSourceSchemaVersion):
		code = "unsupported_schema"
	}

	t.mu.Lock()
	t.failures = append(t.failures, refreshContentStatsFailure{BookID: bookID, Code: code})
	t.mu.Unlock()
	t.logger.Error("failed to refresh content stats", "shelf_id", t.shelfID, "book_id", bookID, "error", err)
}

func (t *refreshContentStatsTask) collectBookIDsNeedingContentStats(books []*shelf.Book) []string {
	ids := make([]string, 0)
	for _, book := range books {
		source, err := book.GetSource(book.CurrentSource())
		if err != nil {
			ids = append(ids, book.ID())
			continue
		}
		if source.GetMeta().CharCount == 0 {
			ids = append(ids, book.ID())
		}
	}
	return ids
}

func (t *refreshContentStatsTask) refreshCurrentSourceContentStats(bookID string) error {
	book, err := t.shelf.GetBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := source.RefreshContentMetadata(); err != nil {
		return util.Errorf("%w", err)
	}

	t.shelf.RefreshBookCharCount(bookID)
	return nil
}

func (t *refreshContentStatsTask) Run(ctx context.Context) error {
	t.progress.SetStatus(taskutil.StatusRunning)

	books, err := t.shelf.ListBooks()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list books", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	bookIDs := t.collectBookIDsNeedingContentStats(t.visible.Only(books))
	t.progress.SetTotal(len(bookIDs))
	if len(bookIDs) == 0 {
		t.progress.SetStatus(taskutil.StatusCompleted)
		return nil
	}

	for _, bookID := range bookIDs {
		if err := ctx.Err(); err != nil {
			t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
			t.logger.Info("content stats refresh cancelled", "shelf_id", t.shelfID, "book_id", bookID)
			return util.Errorf("%w", err)
		}

		if refreshErr := t.refreshCurrentSourceContentStats(bookID); refreshErr != nil {
			t.recordFailure(bookID, refreshErr)
		} else {
			t.mu.Lock()
			t.refreshed++
			t.mu.Unlock()
		}

		t.progress.Advance()
	}

	t.mu.Lock()
	failed := len(t.failures)
	t.mu.Unlock()
	return settleFinishedSweep(&t.progress, failed)
}

func NewRefreshContentStatsChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger, visible VisibleBooks) *taskutil.TaskChain {
	return &taskutil.TaskChain{
		Key:         RefreshContentStatsTaskName + ":" + shelfID,
		Name:        RefreshContentStatsTaskName,
		Title:       "Update content statistics",
		Description: "Recompute content statistics for books with an unknown character count",
		Tasks:       []taskutil.Task{newRefreshContentStatsTask(shelfID, s, logger, visible)},
	}
}
