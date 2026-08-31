// Package readingclose persists the reader's last position when the desktop app
// or the standalone reader window is closed.
//
// The frontend autosaves reading progress to the shared file on a 10s interval,
// so closing the window between ticks would drop up to the last ten seconds of
// reading. Asking the frontend to flush over an async event at close time proved
// unreliable during window teardown, so instead the frontend continuously
// *stages* its latest position here (a cheap in-memory report), and the window's
// OnBeforeClose writes that staged position to disk synchronously — a direct Go
// write it can guarantee completes before the window goes down.
//
// The frontend still owns the reading-progress document and its autosave; the
// Stager only captures the few seconds a close would otherwise drop, and folds
// them in newest-wins (readingprogress.MergeNewest) so it never clobbers a newer
// entry another process wrote.
//
// Both the desktop app and the reader are independent Wails modules; this is the
// close-time persistence they share.
package readingclose

import (
	"log"
	"maps"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/readingprogress"
)

// DefaultTimeout bounds how long the close-time write may run before the window
// is allowed to close regardless. It has to cover a real save — the shared store
// is a read-modify-write behind a cross-process flock that can queue behind the
// other app — without letting a contended lock hold the window open: the common,
// uncontended case completes in well under this.
const DefaultTimeout = 1500 * time.Millisecond

// Stager buffers the latest reading position reported by the frontend and writes
// it to the shared reading-progress file on window close.
type Stager struct {
	store   *readingprogress.Store
	timeout time.Duration

	mu     sync.Mutex
	staged map[string]map[string]readingprogress.Entry
}

// NewStager builds a Stager that writes through store (the same shared store the
// app autosaves to) and waits at most timeout for the close-time write. A nil
// store makes Stage and PersistOnClose no-ops, matching an app whose config
// directory could not be resolved and so persists no progress.
func NewStager(store *readingprogress.Store, timeout time.Duration) *Stager {
	return &Stager{
		store:   store,
		timeout: timeout,
		staged:  map[string]map[string]readingprogress.Entry{},
	}
}

// Stage records the latest position for one book under its shelf. It is called
// on every position change, so it only touches memory and returns at once; the
// disk write happens once, on close. A later Stage for the same book replaces
// the earlier one — only the position at close time matters.
func (s *Stager) Stage(shelfID, bookID string, offset, at int64) {
	if s == nil || shelfID == "" || bookID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	books := s.staged[shelfID]
	if books == nil {
		books = map[string]readingprogress.Entry{}
		s.staged[shelfID] = books
	}
	books[bookID] = readingprogress.Entry{Offset: offset, At: at}
}

// PersistOnClose folds the staged positions into the shared file. The write runs
// on its own goroutine and PersistOnClose waits at most timeout for it, so a
// contended cross-process lock cannot keep the window from closing. It is meant
// to be called from OnBeforeClose before it allows the close.
func (s *Stager) PersistOnClose() {
	if s == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.persist()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(s.timeout):
	}
}

func (s *Stager) persist() {
	if s.store == nil {
		return
	}
	staged := s.snapshot()
	if len(staged.Shelves) == 0 {
		return
	}
	if _, err := s.store.Mutate(func(disk readingprogress.Document) readingprogress.Document {
		return readingprogress.MergeNewest(disk, staged)
	}); err != nil {
		log.Println("failed to persist reading progress on close:", err)
	}
}

// snapshot copies the staged positions under the lock so the write cannot race a
// concurrent Stage.
func (s *Stager) snapshot() readingprogress.Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := readingprogress.New()
	for shelfID, books := range s.staged {
		if len(books) == 0 {
			continue
		}
		doc.Shelves[shelfID] = maps.Clone(books)
	}
	return doc
}
