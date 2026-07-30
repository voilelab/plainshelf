package server

import (
	"context"
	"strconv"
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/taskutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

const emptyTrashTaskName = "empty_trash"

// emptyTrashTask permanently deletes every book in a shelf's trash.
//
// Books are deleted one at a time through the regular single-book primitive
// rather than under one long-held shelf lock: the lock is a flock with a finite
// timeout, and holding it for the whole sweep would block every read for as
// long as the trash takes to empty.
type emptyTrashTask struct {
	shelfID string
	shelf   *shelf.Shelf
	logger  *logutil.Logger

	progress taskutil.Progress

	mu      sync.Mutex
	deleted int
	failed  int
}

func newEmptyTrashTask(shelfID string, s *shelf.Shelf, logger *logutil.Logger) *emptyTrashTask {
	return &emptyTrashTask{shelfID: shelfID, shelf: s, logger: logger}
}

func (t *emptyTrashTask) Name() string { return emptyTrashTaskName }

func (t *emptyTrashTask) Title() string { return "Empty trash" }

func (t *emptyTrashTask) Description() string {
	t.mu.Lock()
	deleted, failed := t.deleted, t.failed
	t.mu.Unlock()

	_, total := t.progress.Counts()

	desc := "deleted " + strconv.Itoa(deleted) + " of " + strconv.Itoa(total) + " books"
	if failed > 0 {
		desc += ", " + strconv.Itoa(failed) + " failed"
	}
	return desc
}

func (t *emptyTrashTask) Percentage() float64 { return t.progress.Percentage() }

func (t *emptyTrashTask) Status() taskutil.Status { return t.progress.Status() }

func (t *emptyTrashTask) Run(ctx context.Context) error {
	t.progress.SetStatus(taskutil.StatusRunning)

	// A failure here means the deletion never started at all, for example
	// because the trash directory could not be read or the shelf lock timed
	// out. That is the only outcome reported as an outright failure.
	ids, err := t.shelf.ListTrashedBookIDs()
	if err != nil {
		t.progress.SetStatus(taskutil.StatusFailed)
		t.logger.Error("failed to list trashed books", "shelf_id", t.shelfID, "error", err)
		return util.Errorf("%w", err)
	}

	t.progress.SetTotal(len(ids))
	if len(ids) == 0 {
		t.progress.SetStatus(taskutil.StatusCompleted)
		return nil
	}

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
			t.logger.Info("empty trash cancelled", "shelf_id", t.shelfID, "book_id", id)
			return util.Errorf("%w", err)
		}

		if err := t.shelf.DeleteTrashedBook(id); err != nil {
			t.mu.Lock()
			t.failed++
			t.mu.Unlock()
			t.logger.Error("failed to permanently delete trashed book", "shelf_id", t.shelfID, "book_id", id, "error", err)
		} else {
			t.mu.Lock()
			t.deleted++
			t.mu.Unlock()
		}

		// Advance on failure too: the percentage tracks books processed, so it
		// still reaches 100% when some deletions could not be completed.
		t.progress.Advance()
	}

	t.mu.Lock()
	failed := t.failed
	t.mu.Unlock()

	if failed > 0 {
		t.progress.SetStatus(taskutil.StatusPartiallyCompleted)
		// The chain itself did run, so do not report an error and abort it.
		return nil
	}

	t.progress.SetStatus(taskutil.StatusCompleted)
	return nil
}

func newEmptyTrashChain(shelfID string, s *shelf.Shelf, logger *logutil.Logger) *taskutil.TaskChain {
	return &taskutil.TaskChain{
		Key:         emptyTrashTaskName + ":" + shelfID,
		Name:        emptyTrashTaskName,
		Title:       "Empty trash",
		Description: "Permanently delete every book in the trash",
		Tasks:       []taskutil.Task{newEmptyTrashTask(shelfID, s, logger)},
	}
}
