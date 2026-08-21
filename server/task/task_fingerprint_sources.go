package task

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const FingerprintSourcesTaskName = "fingerprint_sources"

type fingerprintSourceFailure struct {
	BookID string `json:"book_id"`

	// SourceID is empty when the book's sources could not be listed at all, so
	// there is no single source to name.
	SourceID string `json:"source_id,omitempty"`

	Code string `json:"code"`
}

// FingerprintSourcesResult reports what one sweep cost, split by how each source
// was answered. Reused is the number that never left the cache, which is what
// makes a repeat sweep observably cheap rather than merely fast.
//
// Total counts the sources the sweep set out to visit, so Computed, Deduped,
// Reused and the per-source failures add up to it. A book whose sources could
// not be listed at all is in Failures without being in Total: it never produced
// a source to count.
type FingerprintSourcesResult struct {
	Total    int                        `json:"total"`
	Computed int                        `json:"computed"`
	Deduped  int                        `json:"deduped"`
	Reused   int                        `json:"reused"`
	Failures []fingerprintSourceFailure `json:"failures"`
}

// fingerprintSourcesTask builds the similarity fingerprint of every source in
// the shelf, reading only the ones the fingerprint cache cannot already answer.
//
// The shape follows refreshContentStatsTask: sources are processed one at a
// time through the regular shelf primitives rather than under one long-held
// shelf lock, because the lock is a flock with a finite timeout and holding it
// for the whole sweep would block every read for as long as the sweep takes. A
// source that cannot be read is recorded in failures and the sweep continues.
//
// It runs sequentially. The work is I/O bound, a shelf on SMB is exactly where
// parallel reads stop helping, and the worker already runs one chain at a time.
type fingerprintSourcesTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger

	progress taskutil.Progress

	mu       sync.Mutex
	computed int
	deduped  int
	reused   int
	failures []fingerprintSourceFailure
}

func newFingerprintSourcesTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger) *fingerprintSourcesTask {
	return &fingerprintSourcesTask{
		shelfID:  shelfID,
		shelf:    s,
		logger:   logger,
		failures: []fingerprintSourceFailure{},
	}
}

func (t *fingerprintSourcesTask) Name() string {
	return FingerprintSourcesTaskName
}

func (t *fingerprintSourcesTask) Title() string {
	return "Build content fingerprints"
}

func (t *fingerprintSourcesTask) Description() string {
	t.mu.Lock()
	computed, deduped, reused, failed := t.computed, t.deduped, t.reused, len(t.failures)
	t.mu.Unlock()

	_, total := t.progress.Counts()

	desc := "fingerprinted " + strconv.Itoa(computed+deduped) + " of " + strconv.Itoa(total) + " sources"
	if reused > 0 {
		desc += ", " + strconv.Itoa(reused) + " unchanged"
	}
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
		Total:    total,
		Computed: t.computed,
		Deduped:  t.deduped,
		Reused:   t.reused,
		Failures: append(make([]fingerprintSourceFailure, 0, len(t.failures)), t.failures...),
	}
}

func (t *fingerprintSourcesTask) recordOutcome(outcome shelf.FingerprintOutcome) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch outcome {
	case shelf.FingerprintComputed:
		t.computed++
	case shelf.FingerprintDeduped:
		t.deduped++
	case shelf.FingerprintReused:
		t.reused++
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
	t.failures = append(t.failures, fingerprintSourceFailure{BookID: bookID, SourceID: sourceID, Code: code})
	t.mu.Unlock()
	t.logger.Error("failed to fingerprint source", "shelf_id", t.shelfID, "book_id", bookID, "source_id", sourceID, "error", err)
}

func (t *fingerprintSourcesTask) failedCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.failures)
}

func (t *fingerprintSourcesTask) Run(ctx context.Context) error {
	t.progress.SetStatus(taskutil.StatusRunning)

	// The sweep's whole product is the cache it writes, so a shelf that cannot
	// be written to is refused before any file is read rather than reported as a
	// sweep that succeeded and kept nothing. The submitting endpoint refuses
	// first; this keeps that true for any other caller.
	if t.shelf.ReadOnly() {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("cannot fingerprint a read-only shelf", "shelf_id", t.shelfID)
		return util.Errorf("%w", fsutil.ErrReadOnly)
	}

	// A failure here means the sweep never started at all, for example because
	// the shelf is still initializing or its lock timed out. That is the only
	// outcome reported as an outright failure.
	targets, unlistable, err := t.shelf.FingerprintTargets()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list sources to fingerprint", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	for _, bookID := range unlistable {
		t.recordFailure(bookID, "", util.NewError("sources could not be listed"))
	}

	t.progress.SetTotal(len(targets))

	cache := t.shelf.LoadFingerprintCache()

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			// What has been fingerprinted so far is still worth keeping: it is
			// what the next sweep does not have to read again.
			t.persist(cache)
			t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
			t.logger.Info("fingerprint sweep cancelled", "shelf_id", t.shelfID, "book_id", target.BookID, "source_id", target.SourceID)
			return util.Errorf("%w", err)
		}

		outcome, fingerprintErr := t.shelf.Fingerprint(cache, target)
		if fingerprintErr != nil {
			t.recordFailure(target.BookID, target.SourceID, fingerprintErr)
		} else {
			t.recordOutcome(outcome)
		}

		// Advance on failure too: the percentage tracks sources processed, so it
		// still reaches 100% when some of them could not be read.
		t.progress.Advance()
	}

	// Written once at the end rather than per source. The file is rewritten
	// whole, so a sweep that saved as it went would rewrite the entire cache
	// once per book.
	if err := t.shelf.SaveFingerprintCache(cache); err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to save the fingerprint cache", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	if t.failedCount() > 0 {
		// The chain itself did run, so do not report an error and abort it.
		t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
		return nil
	}

	t.progress.SetStatus(taskutil.StatusCompleted)
	return nil
}

// persist saves the cache on a path that is already returning an error, where
// failing to write it changes nothing the caller can act on.
func (t *fingerprintSourcesTask) persist(cache *shelf.FingerprintCache) {
	if err := t.shelf.SaveFingerprintCache(cache); err != nil {
		t.logger.Error("failed to save the fingerprint cache", "shelf_id", t.shelfID, "error", err)
	}
}

func NewFingerprintSourcesChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger) *taskutil.TaskChain {
	return &taskutil.TaskChain{
		Key:         FingerprintSourcesTaskName + ":" + shelfID,
		Name:        FingerprintSourcesTaskName,
		Title:       "Build content fingerprints",
		Description: "Fingerprint every source that has changed since the last sweep",
		Tasks:       []taskutil.Task{newFingerprintSourcesTask(shelfID, s, logger)},
	}
}
