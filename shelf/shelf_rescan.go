package shelf

import (
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

// ErrRescanInProgress reports that a manual rescan is already walking this
// shelf, so this one was refused rather than started alongside it.
//
// Refused rather than queued or attached: a second walk would cost the same
// directory traversal to answer a question the first walk is already
// answering, and on the SMB and cloud-mounted shelves this button exists for,
// that traversal is the expensive part. Attaching to the running walk was the
// alternative, and it is worse for the reason the button exists at all — a walk
// that began before the user dropped their file in cannot report the file, so
// attaching would answer "scanned, still not there" while a retry after it ends
// finds the book.
var ErrRescanInProgress = util.NewError("a rescan is already in progress")

// RescanResult reports what a manual rescan found.
type RescanResult struct {
	// ID names this rescan while it runs. It is also filled in on the
	// ErrRescanInProgress failure, where it names the rescan that was already
	// running, so a caller can tell its own user which one to wait for.
	ID string

	// StartedAt is when the walk began, not when it finished; see
	// scanToBookCache for why the shelf's freshness is dated from the start.
	StartedAt time.Time

	BookCount  int
	LayerCount int
}

// Rescan walks the shelf now and rebuilds the book cache from what it finds.
//
// scanInterval does not apply. It exists to stop repeated browsing from
// re-walking the tree, and a user who pressed a button has already answered the
// question it asks. This is the one unconditional full scan the API can reach
// without a side effect: ExportBookCache also forces a walk, but it writes a
// file afterwards, which makes it the wrong thing to reach for when all that is
// wanted is for a book dropped into books/ to show up.
func (s *Shelf) Rescan() (RescanResult, error) {
	if !s.IsReady() {
		return RescanResult{}, util.Errorf("%w", ErrShelfInitializing)
	}

	scanID, runningID := s.beginRescan()
	if scanID == "" {
		return RescanResult{ID: runningID}, util.Errorf("%w", ErrRescanInProgress)
	}
	defer s.endRescan()

	// The shared lock is the one every read takes, so holding it for the walk
	// leaves the library readable throughout. It excludes only the structural
	// writes that would move a book out from under the walk.
	if err := s.shelfLock.RLock(); err != nil {
		return RescanResult{}, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	if err := s.scanToBookCache(); err != nil {
		return RescanResult{}, util.Errorf("%w", err)
	}

	// Both counts come from the cache the walk just filled, which is also why
	// the layer count includes a folder holding no books: the walk reports every
	// layer directory it enters, and a layer the user made but has not filed
	// anything into is still part of the shelf this number describes.
	s.bookCache.RLock()
	defer s.bookCache.RUnlock()

	return RescanResult{
		ID:         scanID,
		StartedAt:  s.bookCache.lastScanStart,
		BookCount:  len(s.bookCache.cache),
		LayerCount: len(s.bookCache.layers),
	}, nil
}

// beginRescan claims the shelf for one rescan. It returns the new scan's ID and
// an empty running ID on success, or an empty scan ID and the ID of the rescan
// already holding the shelf.
func (s *Shelf) beginRescan() (scanID, runningID string) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	if s.bookCache.rescanID != "" {
		return "", s.bookCache.rescanID
	}

	// Not a cryptographic identifier: it is never persisted and never
	// authenticates anything. It exists so a refused caller can say which
	// rescan it lost to, and so two of them can be told apart in the log.
	s.bookCache.rescanID = randomString(12)
	return s.bookCache.rescanID, ""
}

func (s *Shelf) endRescan() {
	s.bookCache.Lock()
	s.bookCache.rescanID = ""
	s.bookCache.Unlock()
}
