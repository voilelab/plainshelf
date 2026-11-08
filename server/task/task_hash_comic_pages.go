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
	"github.com/voilelab/plainshelf/shelf/pagehash"
)

const HashComicPagesTaskName = "hash_comic_pages"

type hashComicPagesFailure struct {
	BookID string `json:"book_id"`
	Code   string `json:"code"`
}

type HashComicPagesResult struct {
	Books  int `json:"books"`
	Comics int `json:"comics"`
	Hashed int `json:"hashed"`

	Pages     int `json:"pages"`
	StatHits  int `json:"stat_hits"`
	PagesRead int `json:"pages_read"`

	Failures []hashComicPagesFailure `json:"failures"`
}

type hashComicPagesTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger
	visible VisibleBooks

	progress taskutil.Progress

	mu       sync.Mutex
	comics   int
	hashed   int
	stats    shelf.PageHashCacheStats
	failures []hashComicPagesFailure
}

func (t *hashComicPagesTask) Name() string {
	return HashComicPagesTaskName
}

func (t *hashComicPagesTask) Title() string {
	return "Hash comic pages"
}

func (t *hashComicPagesTask) Description() string {
	t.mu.Lock()
	hashed, failed := t.hashed, len(t.failures)
	t.mu.Unlock()

	done, total := t.progress.Counts()
	desc := "hashed " + strconv.Itoa(hashed) + " comics in " +
		strconv.Itoa(done) + " of " + strconv.Itoa(total) + " books"
	if failed > 0 {
		desc += ", " + strconv.Itoa(failed) + " failed"
	}
	return desc
}

func (t *hashComicPagesTask) Percentage() float64 {
	return t.progress.Percentage()
}

func (t *hashComicPagesTask) Status() taskutil.Status {
	return t.progress.Status()
}

func (t *hashComicPagesTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, total := t.progress.Counts()
	return HashComicPagesResult{
		Books:     total,
		Comics:    t.comics,
		Hashed:    t.hashed,
		Pages:     t.stats.Pages,
		StatHits:  t.stats.StatHits,
		PagesRead: t.stats.Hashed,
		Failures:  append(make([]hashComicPagesFailure, 0, len(t.failures)), t.failures...),
	}
}

func (t *hashComicPagesTask) recordFailure(bookID string, err error) {
	code := "hash_failed"
	switch {
	case errors.Is(err, shelf.ErrBookNotFound), errors.Is(err, shelf.ErrSourceNotFound):
		code = "not_found"
	case errors.Is(err, pagehash.ErrMissingPages):
		code = "missing_pages"
	case errors.Is(err, shelf.ErrUnsupportedBookSchemaVersion), errors.Is(err, shelf.ErrUnsupportedSourceSchemaVersion):
		code = "unsupported_schema"
	}

	t.mu.Lock()
	t.failures = append(t.failures, hashComicPagesFailure{BookID: bookID, Code: code})
	t.mu.Unlock()
	t.logger.Error("failed to hash comic pages", "shelf_id", t.shelfID, "book_id", bookID, "error", err)
}

func (t *hashComicPagesTask) hashBook(cache *shelf.PageHashCache, bookID string) {
	book, err := t.shelf.GetBook(bookID)
	if err != nil {
		t.recordFailure(bookID, util.Errorf("%w", err))
		return
	}
	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.recordFailure(bookID, util.Errorf("%w", err))
		return
	}
	if source.HoldsText() {
		return
	}

	t.mu.Lock()
	t.comics++
	t.mu.Unlock()

	if _, err := cache.Resolve(bookID, source); err != nil {
		t.recordFailure(bookID, err)
		return
	}

	t.mu.Lock()
	t.hashed++
	t.mu.Unlock()
}

func (t *hashComicPagesTask) save(cache *shelf.PageHashCache) error {
	stats := cache.Stats()
	t.mu.Lock()
	t.stats = stats
	t.mu.Unlock()

	if err := cache.Save(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (t *hashComicPagesTask) Run(ctx context.Context) error {
	t.progress.SetStatus(taskutil.StatusRunning)

	books, err := t.shelf.ListBooks()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list books", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	cache := t.shelf.OpenPageHashCache()
	bookIDs := snapshotBookIDs(t.visible.Only(books))
	t.progress.SetTotal(len(bookIDs))

	for _, bookID := range bookIDs {
		if err := ctx.Err(); err != nil {
			if saveErr := t.save(cache); saveErr != nil {
				t.logger.Error("failed to save the page hash cache after cancellation", "shelf_id", t.shelfID, "error", saveErr)
			}
			t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
			return util.Errorf("%w", err)
		}
		t.hashBook(cache, bookID)
		t.progress.Advance()
	}

	if err := t.save(cache); err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to save the page hash cache", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	t.mu.Lock()
	failed := len(t.failures)
	t.mu.Unlock()
	return settleFinishedSweep(&t.progress, failed)
}

func NewHashComicPagesChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger, visible VisibleBooks) *taskutil.TaskChain {
	return &taskutil.TaskChain{
		Key:         HashComicPagesTaskName + ":" + shelfID,
		Name:        HashComicPagesTaskName,
		Title:       "Hash comic pages",
		Description: "Hash every comic page that has changed since the last run",
		Tasks: []taskutil.Task{&hashComicPagesTask{
			shelfID:  shelfID,
			shelf:    s,
			logger:   logger,
			visible:  visible,
			failures: []hashComicPagesFailure{},
		}},
	}
}
