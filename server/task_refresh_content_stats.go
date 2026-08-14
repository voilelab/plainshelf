package server

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

const refreshContentStatsTaskName = "refresh_content_stats"

type refreshContentStatsFailure struct {
	BookID string `json:"book_id"`
	Code   string `json:"code"`
}

type refreshContentStatsResult struct {
	Total     int                          `json:"total"`
	Refreshed int                          `json:"refreshed"`
	Failures  []refreshContentStatsFailure `json:"failures"`
}

// refreshContentStatsTask recomputes the content metadata of every book whose
// current source reports no character count.
//
// A stored count of 0 is what an unknown count looks like on disk: SourceMeta
// carries CharCount as a plain int with omitempty, so a book whose count was
// never computed is indistinguishable from a genuinely empty one. Both are
// cheap to recompute and land on the same maintenance page, so both are swept.
//
// Sources are refreshed one at a time through the regular single-source
// primitive rather than under one long-held shelf lock, matching emptyTrashTask:
// the lock is a flock with a finite timeout, and holding it for the whole sweep
// would block every read for as long as the sweep takes.
type refreshContentStatsTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger

	progress taskutil.Progress

	mu        sync.Mutex
	refreshed int
	failures  []refreshContentStatsFailure
}

func newRefreshContentStatsTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger) *refreshContentStatsTask {
	return &refreshContentStatsTask{
		shelfID:  shelfID,
		shelf:    s,
		logger:   logger,
		failures: []refreshContentStatsFailure{},
	}
}

func (t *refreshContentStatsTask) Name() string { return refreshContentStatsTaskName }

func (t *refreshContentStatsTask) Title() string { return "Update content statistics" }

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

func (t *refreshContentStatsTask) Percentage() float64 { return t.progress.Percentage() }

func (t *refreshContentStatsTask) Status() taskutil.Status { return t.progress.Status() }

func (t *refreshContentStatsTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, total := t.progress.Counts()
	return refreshContentStatsResult{
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

// candidate pairs a book with the source that needs recomputing. srcErr records
// a source that could not be opened at all: such a book still counts towards the
// sweep so the user sees it reported rather than silently skipped.
type refreshContentStatsCandidate struct {
	bookID string
	source *shelf.Source
	srcErr error
}

// collectCandidates opens each book's current source to read its stored count.
// This only touches meta.json, the same work GET /books?include=char_count
// already does, so it is far cheaper than the recompute pass that follows and
// lets the task report an accurate total before it starts.
func (t *refreshContentStatsTask) collectCandidates(books []*shelf.Book) []refreshContentStatsCandidate {
	candidates := make([]refreshContentStatsCandidate, 0)
	for _, book := range books {
		source, err := book.GetSource(book.CurrentSource())
		if err != nil {
			candidates = append(candidates, refreshContentStatsCandidate{bookID: book.ID(), srcErr: err})
			continue
		}
		if source.GetMeta().CharCount == 0 {
			candidates = append(candidates, refreshContentStatsCandidate{bookID: book.ID(), source: source})
		}
	}
	return candidates
}

func (t *refreshContentStatsTask) Run(ctx context.Context) error {
	t.progress.SetStatus(taskutil.StatusRunning)

	// A failure here means the sweep never started at all, for example because
	// the shelf is still initializing or its lock timed out. That is the only
	// outcome reported as an outright failure.
	books, err := t.shelf.ListBooks()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list books", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	candidates := t.collectCandidates(books)
	t.progress.SetTotal(len(candidates))
	if len(candidates) == 0 {
		t.progress.SetStatus(taskutil.StatusCompleted)
		return nil
	}

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
			t.logger.Info("content stats refresh cancelled", "shelf_id", t.shelfID, "book_id", candidate.bookID)
			return util.Errorf("%w", err)
		}

		refreshErr := candidate.srcErr
		if refreshErr == nil {
			refreshErr = candidate.source.RefreshContentMetadata()
		}

		if refreshErr != nil {
			t.recordFailure(candidate.bookID, refreshErr)
		} else {
			t.mu.Lock()
			t.refreshed++
			t.mu.Unlock()
		}

		// Advance on failure too: the percentage tracks books processed, so it
		// still reaches 100% when some sources could not be recomputed.
		t.progress.Advance()
	}

	t.mu.Lock()
	failed := len(t.failures)
	t.mu.Unlock()

	if failed > 0 {
		// The chain itself did run, so do not report an error and abort it.
		t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
		return nil
	}

	t.progress.SetStatus(taskutil.StatusCompleted)
	return nil
}

func newRefreshContentStatsChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger) *taskutil.TaskChain {
	return &taskutil.TaskChain{
		Key:         refreshContentStatsTaskName + ":" + shelfID,
		Name:        refreshContentStatsTaskName,
		Title:       "Update content statistics",
		Description: "Recompute content statistics for books with an unknown character count",
		Tasks:       []taskutil.Task{newRefreshContentStatsTask(shelfID, s, logger)},
	}
}
