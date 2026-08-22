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

// FingerprintAlgo names the rules BuildFingerprint follows, for the cache that
// stores its answers.
//
// It lives here rather than in the shelf package because shelf stores a
// fingerprint without knowing how one is computed, and here rather than in
// internal/sketch because the version strings come from two packages that do
// not know about each other. Everything that reads a stored fingerprint has to
// open the cache with this exact value: a mismatch discards the whole file, so
// two callers naming the algorithm differently would rebuild it in turn,
// forever.
func FingerprintAlgo() shelf.FingerprintAlgo {
	return shelf.FingerprintAlgo{
		Normalize: textnorm.NormalizeVersion,
		Shingle:   "char-" + strconv.Itoa(sketch.DefaultN) + "gram",
		Hash:      textnorm.Hash64Version,
		K:         sketch.DefaultK,
	}
}

// BuildFingerprint reduces one source's bytes to the fingerprint stored for it.
//
// Everything is derived from the single buffer the cache has already read, in
// one pass per metric, for the reason refreshContentMetadata gives: on a
// network-mounted shelf each extra read of source.txt is another round trip.
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

	// SourceID is empty when the book itself could not be opened or listed, in
	// which case none of its sources was reached.
	SourceID string `json:"source_id,omitempty"`

	Code string `json:"code"`
}

// FingerprintSourcesResult reports both what was covered and what it cost. The
// cache counters are part of the result rather than a debug log because "the
// second run reads nothing" is the property this task exists for, and wall
// time is not evidence of it.
type FingerprintSourcesResult struct {
	Books         int `json:"books"`
	Sources       int `json:"sources"`
	Fingerprinted int `json:"fingerprinted"`

	// StatHits is how many sources were answered from the index without being
	// opened, ContentHits how many were read but recognized by content, Built
	// how many fingerprints had to be computed, and Repaired how many sources
	// had an md5_hash that disagreed with their content.
	StatHits    int `json:"stat_hits"`
	ContentHits int `json:"content_hits"`
	Built       int `json:"built"`
	Repaired    int `json:"repaired"`

	Failures []fingerprintSourcesFailure `json:"failures"`
}

// fingerprintSourcesTask fingerprints every source of every book, skipping the
// ones the fingerprint cache can already answer for - or, when force is set,
// rebuilding all of them regardless of what the cache holds.
//
// It walks all sources rather than only each book's current source: a reader
// who keeps an earlier import around wants it compared too, and the cache is
// keyed per source, so covering them costs nothing on later runs.
//
// The shape follows refreshContentStatsTask. Books are processed one at a time
// through the regular per-book primitives instead of under one long-held shelf
// lock: the lock is a flock with a finite timeout, and holding it for the whole
// sweep would block every read for as long as the sweep takes. Unlike that
// task, nothing here is skipped on the strength of a stored value - the cache
// decides by comparing a stat against the file itself, so a source edited
// outside PlainShelf cannot hide behind a stale number.
type fingerprintSourcesTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger

	// force ignores the cache and rebuilds every source's fingerprint, for a
	// caller that suspects a stat comparison has missed a content change. When
	// false the sweep stays incremental and skips what the cache can answer.
	force bool

	progress taskutil.Progress

	mu            sync.Mutex
	sources       int
	fingerprinted int
	stats         shelf.FingerprintCacheStats
	failures      []fingerprintSourcesFailure
}

func newFingerprintSourcesTask(shelfID string, s *shelf.Shelf, force bool, logger *logutil.Logger) *fingerprintSourcesTask {
	return &fingerprintSourcesTask{
		shelfID:  shelfID,
		shelf:    s,
		logger:   logger,
		force:    force,
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

// sourceIDs is every source ID of one book, taken from a listing that is then
// dropped.
//
// The listing is not kept because it opens every source at once, and a Source
// carries its whole meta.json in memory and writes all of it back when the
// cache repairs a stale md5_hash. Held while the earlier sources of the book
// are read, the last one's snapshot would be old enough to revert a comment or
// a split config another request stored in the meantime. IDs cannot go stale
// that way, so each source is opened again at the moment it is used.
//
// A source this build cannot open at all - a missing or malformed meta.json -
// is not listed here: ListSource logs it and leaves it out, which is what the
// source list UI shows too. Surfacing it would mean enumerating the directory
// rather than the model, and a source PlainShelf cannot open is invisible
// everywhere in the app, not only in this sweep.
func (t *fingerprintSourcesTask) sourceIDs(book *shelf.Book) ([]string, error) {
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

// fingerprintBook resolves every source of one book, reporting each failure
// separately: one unreadable source must not cost the rest of the book its
// fingerprints, let alone the rest of the sweep.
//
// The book and each of its sources are re-resolved by ID immediately before
// they are read, the way refreshContentStatsTask re-resolves before it writes,
// so this task reads and writes the same meta.json a single-source request
// would.
func (t *fingerprintSourcesTask) fingerprintBook(cache *shelf.FingerprintCache, bookID string) {
	book, err := t.shelf.GetBook(bookID)
	if err != nil {
		t.recordFailure(bookID, "", util.Errorf("%w", err))
		return
	}

	sourceIDs, err := t.sourceIDs(book)
	if err != nil {
		t.recordFailure(bookID, "", err)
		return
	}

	t.mu.Lock()
	t.sources += len(sourceIDs)
	t.mu.Unlock()

	// Rebuild ignores the cache; Resolve skips what it can answer. Both have the
	// same signature, so the choice is made once per book here.
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

// save writes the cache back and folds its counters into the result. It is
// called on the way out of every path that read anything, including a
// cancelled one: the run is incremental, so the fingerprints already computed
// are what the next run will not have to compute again.
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

	// Both of these fail only when the sweep never started at all - the shelf
	// is still initializing, its lock timed out, or the caller named an
	// incomplete algorithm - which is the only outcome reported as an outright
	// failure.
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

	// Book IDs rather than the listed books: the sweep re-resolves each one
	// when it reaches it, so a book moved or retitled while the sweep runs is
	// read where it is now.
	bookIDs := make([]string, 0, len(books))
	for _, book := range books {
		bookIDs = append(bookIDs, book.ID())
	}

	t.progress.SetTotal(len(bookIDs))

	for _, bookID := range bookIDs {
		if err := ctx.Err(); err != nil {
			// Saved before returning: a cancelled run still keeps whatever it
			// learned, which is the point of an incremental task.
			if saveErr := t.save(cache); saveErr != nil {
				t.logger.Error("failed to save the fingerprint cache after cancellation",
					"shelf_id", t.shelfID, "error", saveErr)
			}
			t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
			t.logger.Info("source fingerprinting cancelled", "shelf_id", t.shelfID, "book_id", bookID)
			return util.Errorf("%w", err)
		}

		t.fingerprintBook(cache, bookID)

		// Advance on failure too: the percentage tracks books processed, so it
		// still reaches 100% when some sources could not be fingerprinted.
		t.progress.Advance()
	}

	// A shelf that cannot be written to is reported as a failure rather than
	// swallowed: everything this run computed would be lost, and a task that
	// answered "completed" would invite the user to run it again for the same
	// result. The endpoint refuses a read-only shelf before it gets here; this
	// covers a shelf that became unwritable while the sweep ran.
	if err := t.save(cache); err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to save the fingerprint cache", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
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

func NewFingerprintSourcesChain(shelfID string, s *shelf.Shelf, force bool, logger *logutil.Logger) *taskutil.TaskChain {
	// The key does not vary with force: an incremental sweep and a forced one
	// both walk the whole shelf and write the same cache, so only one may run at
	// a time, and a second request is pointed at the running chain either way.
	description := "Fingerprint every source that has changed since the last run"
	if force {
		description = "Rebuild every source's fingerprint, ignoring the cache"
	}

	return &taskutil.TaskChain{
		Key:         FingerprintSourcesTaskName + ":" + shelfID,
		Name:        FingerprintSourcesTaskName,
		Title:       "Build source fingerprints",
		Description: description,
		Tasks:       []taskutil.Task{newFingerprintSourcesTask(shelfID, s, force, logger)},
	}
}
