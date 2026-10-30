package task

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/sketch"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/textnorm"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const FingerprintSourcesTaskName = "fingerprint_sources"

func FingerprintAlgo() shelf.FingerprintAlgo {
	return shelf.FingerprintAlgo{
		Normalize: textnorm.NormalizeVersion,
		Shingle:   "char-" + strconv.Itoa(sketch.DefaultN) + "gram",
		Hash:      textnorm.Hash64Version,
		K:         sketch.DefaultK,
	}
}

func BuildFingerprint(content []byte) (shelf.FingerprintEntry, error) {
	normalized := textnorm.Normalize(string(content))

	normMD5, err := hashutil.MD5Hash(strings.NewReader(normalized))
	if err != nil {
		return shelf.FingerprintEntry{}, util.Errorf("%w", err)
	}

	built := sketch.BuildDefault(normalized)
	return shelf.FingerprintEntry{
		NormMD5:   normMD5,
		NormChars: utf8.RuneCountInString(normalized),
		Shingles:  built.Distinct,
		Sketch:    built.Encode(),
	}, nil
}

type fingerprintSourcesFailure struct {
	BookID string `json:"book_id"`

	SourceID string `json:"source_id,omitempty"`

	Code string `json:"code"`
}

type FingerprintSourcesResult struct {
	Books         int `json:"books"`
	Sources       int `json:"sources"`
	Fingerprinted int `json:"fingerprinted"`

	StatHits    int `json:"stat_hits"`
	ContentHits int `json:"content_hits"`
	Built       int `json:"built"`
	Repaired    int `json:"repaired"`

	Failures []fingerprintSourcesFailure `json:"failures"`
}

type fingerprintSourcesTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger

	force bool

	visible VisibleBooks

	progress taskutil.Progress

	mu            sync.Mutex
	sources       int
	fingerprinted int
	stats         shelf.FingerprintCacheStats
	failures      []fingerprintSourcesFailure
}

func newFingerprintSourcesTask(shelfID string, s *shelf.Shelf, force bool, logger *logutil.Logger, visible VisibleBooks) *fingerprintSourcesTask {
	return &fingerprintSourcesTask{
		shelfID:  shelfID,
		shelf:    s,
		logger:   logger,
		force:    force,
		visible:  visible,
		failures: []fingerprintSourcesFailure{},
	}
}

func (t *fingerprintSourcesTask) Name() string {
	return FingerprintSourcesTaskName
}

func (t *fingerprintSourcesTask) Title() string {
	return "Build source fingerprints"
}

func (t *fingerprintSourcesTask) Description() string {
	t.mu.Lock()
	fingerprinted, failed := t.fingerprinted, len(t.failures)
	t.mu.Unlock()

	done, total := t.progress.Counts()

	desc := "fingerprinted " + strconv.Itoa(fingerprinted) + " sources in " +
		strconv.Itoa(done) + " of " + strconv.Itoa(total) + " books"
	if failed > 0 {
		desc += ", " + strconv.Itoa(failed) + " failed"
	}
	return desc
}

func (t *fingerprintSourcesTask) Percentage() float64 {
	return t.progress.Percentage()
}

func (t *fingerprintSourcesTask) Status() taskutil.Status {
	return t.progress.Status()
}

func (t *fingerprintSourcesTask) Result() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, total := t.progress.Counts()
	return FingerprintSourcesResult{
		Books:         total,
		Sources:       t.sources,
		Fingerprinted: t.fingerprinted,
		StatHits:      t.stats.StatHits,
		ContentHits:   t.stats.ContentHits,
		Built:         t.stats.Built,
		Repaired:      t.stats.Repaired,
		Failures:      append(make([]fingerprintSourcesFailure, 0, len(t.failures)), t.failures...),
	}
}

func (t *fingerprintSourcesTask) recordFailure(bookID, sourceID string, err error) {
	code := "fingerprint_failed"
	switch {
	case errors.Is(err, shelf.ErrBookNotFound), errors.Is(err, shelf.ErrSourceNotFound):
		code = "not_found"
	case errors.Is(err, shelf.ErrUnsupportedBookSchemaVersion), errors.Is(err, shelf.ErrUnsupportedSourceSchemaVersion):
		code = "unsupported_schema"
	}

	t.mu.Lock()
	t.failures = append(t.failures, fingerprintSourcesFailure{BookID: bookID, SourceID: sourceID, Code: code})
	t.mu.Unlock()
	t.logger.Error("failed to fingerprint source",
		"shelf_id", t.shelfID, "book_id", bookID, "source_id", sourceID, "error", err)
}

func (t *fingerprintSourcesTask) sourceIDsToReopen(book *shelf.Book) ([]string, error) {
	sources, err := book.ListSource()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.ID())
	}
	return ids, nil
}

func (t *fingerprintSourcesTask) fingerprintBook(cache *shelf.FingerprintCache, bookID string) {
	book, err := t.shelf.GetBook(bookID)
	if err != nil {
		t.recordFailure(bookID, "", util.Errorf("%w", err))
		return
	}

	sourceIDs, err := t.sourceIDsToReopen(book)
	if err != nil {
		t.recordFailure(bookID, "", err)
		return
	}

	t.mu.Lock()
	t.sources += len(sourceIDs)
	t.mu.Unlock()

	resolve := cache.Resolve
	if t.force {
		resolve = cache.Rebuild
	}

	for _, sourceID := range sourceIDs {
		source, err := book.GetSource(sourceID)
		if err != nil {
			t.recordFailure(bookID, sourceID, util.Errorf("%w", err))
			continue
		}

		if _, err := resolve(book, source, BuildFingerprint); err != nil {
			t.recordFailure(bookID, sourceID, err)
			continue
		}

		t.mu.Lock()
		t.fingerprinted++
		t.mu.Unlock()
	}
}

func (t *fingerprintSourcesTask) save(cache *shelf.FingerprintCache) error {
	stats := cache.Stats()

	t.mu.Lock()
	t.stats = stats
	t.mu.Unlock()

	if err := cache.Save(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (t *fingerprintSourcesTask) Run(ctx context.Context) error {
	t.progress.SetStatus(taskutil.StatusRunning)

	books, err := t.shelf.ListBooks()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list books", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	cache, err := t.shelf.OpenFingerprintCache(FingerprintAlgo())
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to open the fingerprint cache", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	bookIDs := snapshotBookIDs(t.visible.Only(books))
	t.progress.SetTotal(len(bookIDs))

	for _, bookID := range bookIDs {
		if err := ctx.Err(); err != nil {
			return t.cancelKeepingWork(cache, bookID, err)
		}
		t.fingerprintBook(cache, bookID)
		t.progress.Advance()
	}

	if err := t.save(cache); err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to save the fingerprint cache", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	t.mu.Lock()
	failed := len(t.failures)
	t.mu.Unlock()
	return settleFinishedSweep(&t.progress, failed)
}

func (t *fingerprintSourcesTask) cancelKeepingWork(cache *shelf.FingerprintCache, bookID string, cause error) error {
	if err := t.save(cache); err != nil {
		t.logger.Error("failed to save the fingerprint cache after cancellation",
			"shelf_id", t.shelfID, "error", err)
	}
	t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
	t.logger.Info("source fingerprinting cancelled", "shelf_id", t.shelfID, "book_id", bookID)
	return util.Errorf("%w", cause)
}

func fingerprintSourcesChainKey(shelfID string) string {
	return FingerprintSourcesTaskName + ":" + shelfID
}

func NewFingerprintSourcesChain(shelfID string, s *shelf.Shelf, force bool, logger *logutil.Logger, visible VisibleBooks) *taskutil.TaskChain {
	description := "Fingerprint every source that has changed since the last run"
	if force {
		description = "Rebuild every source's fingerprint, ignoring the cache"
	}

	return &taskutil.TaskChain{
		Key:         fingerprintSourcesChainKey(shelfID),
		Name:        FingerprintSourcesTaskName,
		Title:       "Build source fingerprints",
		Description: description,
		Tasks:       []taskutil.Task{newFingerprintSourcesTask(shelfID, s, force, logger, visible)},
	}
}
