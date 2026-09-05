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

	// visible, when non-nil, limits the sweep to these book IDs: the books the
	// request that started it was allowed to see. Nil means it saw the whole
	// shelf, which is every sweep on a server not hiding anything.
	visible map[string]struct{}

	progress taskutil.Progress

	mu        sync.Mutex
	refreshed int
	failures  []refreshContentStatsFailure
}

func newRefreshContentStatsTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger, visible map[string]struct{}) *refreshContentStatsTask {
	return &refreshContentStatsTask{
		shelfID:  shelfID,
		shelf:    s,
		logger:   logger,
		visible:  visible,
		failures: []refreshContentStatsFailure{},
	}
}

// onlyVisible drops the books the request that started this sweep could not
// see, so neither the total it reports nor a failure naming a book ID says
// anything about a book that request was answered 404 for.
//
// The set is a snapshot taken when the chain was submitted, because the sweep
// runs later: a book added since is left to the next sweep rather than swept
// without anyone having decided it is visible.
func (t *refreshContentStatsTask) onlyVisible(books []*shelf.Book) []*shelf.Book {
	if t.visible == nil {
		return books
	}

	kept := make([]*shelf.Book, 0, len(books))
	for _, book := range books {
		if _, ok := t.visible[book.ID()]; ok {
			kept = append(kept, book)
		}
	}
	return kept
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

// collectCandidates opens each book's current source to read its stored count
// and returns the IDs of the books that need recomputing. It deliberately reads
// meta.json rather than the count the book cache holds: the sweep exists to
// repair counts that were never computed, including ones written into the shelf
// from outside, and only the file says what is actually stored. That is still
// far cheaper than the recompute pass that follows, and lets the task report an
// accurate total before it starts.
//
// It deliberately returns IDs rather than the opened sources: a Source carries
// its whole meta.json in memory and writes all of it back, so keeping one for
// the length of the sweep would let this task overwrite a split config or
// comment that another request stored in the meantime.
func (t *refreshContentStatsTask) collectCandidates(books []*shelf.Book) []string {
	ids := make([]string, 0)
	for _, book := range books {
		source, err := book.GetSource(book.CurrentSource())
		if err != nil {
			// A source that cannot be opened has no count to report, so it
			// belongs in the sweep: the recompute pass reports why it failed
			// rather than skipping it silently.
			ids = append(ids, book.ID())
			continue
		}
		if source.GetMeta().CharCount == 0 {
			ids = append(ids, book.ID())
		}
	}
	return ids
}

// refreshBook re-resolves the book and its current source immediately before
// writing, so the sweep reads and writes the same meta.json the single-source
// refresh endpoint would.
func (t *refreshContentStatsTask) refreshBook(bookID string) error {
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

	// A failure here means the sweep never started at all, for example because
	// the shelf is still initializing or its lock timed out. That is the only
	// outcome reported as an outright failure.
	books, err := t.shelf.ListBooks()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list books", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	bookIDs := t.collectCandidates(t.onlyVisible(books))
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

		if refreshErr := t.refreshBook(bookID); refreshErr != nil {
			t.recordFailure(bookID, refreshErr)
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

// NewRefreshContentStatsChain builds the sweep. visible is the set of book IDs
// the requesting client may see, or nil for the whole shelf; see
// refreshContentStatsTask.onlyVisible.
func NewRefreshContentStatsChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger, visible map[string]struct{}) *taskutil.TaskChain {
	return &taskutil.TaskChain{
		Key:         RefreshContentStatsTaskName + ":" + shelfID,
		Name:        RefreshContentStatsTaskName,
		Title:       "Update content statistics",
		Description: "Recompute content statistics for books with an unknown character count",
		Tasks:       []taskutil.Task{newRefreshContentStatsTask(shelfID, s, logger, visible)},
	}
}
