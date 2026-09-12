package readingprogress

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// maxDocumentBytes caps the stored document. It is the same 1 MiB ceiling the
// desktop app has always enforced on its device documents; here it also backstops
// the projection, which never deletes reader entries and so could otherwise grow
// without bound.
const maxDocumentBytes = 1 << 20

const (
	lockTimeout    = 10 * time.Second
	lockRetryDelay = 50 * time.Millisecond
)

// Store is a file-backed reading-progress document guarded by a cross-process
// advisory lock. The desktop app and the standalone reader each open a Store on
// the same path in their own process; the lock (held only for the duration of a
// read-modify-write) serializes their mutations so neither loses the other's
// update. Reads outside a mutation need no lock because every write replaces the
// file atomically.
type Store struct {
	path     string
	lockPath string
}

// NewStore returns a Store for the reading-progress document at path. The lock
// is taken on a sibling ".lock" file rather than the document itself: the
// document is replaced by rename on every write, which would drop a lock held on
// the old inode.
func NewStore(path string) *Store {
	return &Store{path: path, lockPath: path + ".lock"}
}

// Path returns the document path the store manages.
func (s *Store) Path() string { return s.path }

// Read returns the current on-disk document (an empty document when the file
// does not exist yet) together with its raw text. It takes no lock: writes are
// atomic, so a read always sees one whole version or another.
func (s *Store) Read() (Document, string, error) {
	raw, err := readFile(s.path)
	if err != nil {
		return New(), "", err
	}
	return Parse(raw), raw, nil
}

// Mutate applies fn to the current on-disk document under an exclusive
// cross-process lock and atomically persists the result, returning it. When fn
// leaves the document unchanged, nothing is written. fn may be called with an
// empty document (missing file) and must not retain the maps it is handed.
func (s *Store) Mutate(fn func(Document) Document) (Document, error) {
	if s.path == "" {
		return New(), util.NewError("reading progress storage is not ready")
	}

	unlock, err := s.lock()
	if err != nil {
		return New(), err
	}
	defer unlock()

	raw, err := readFile(s.path)
	if err != nil {
		return New(), err
	}
	current := Parse(raw)

	next := fn(current)

	currentText, err := Serialize(current)
	if err != nil {
		return New(), err
	}
	nextText, err := Serialize(next)
	if err != nil {
		return New(), err
	}
	if nextText == currentText {
		return next, nil
	}
	if len(nextText) > maxDocumentBytes {
		return New(), util.Errorf("reading progress document is too large: %d bytes", len(nextText))
	}

	if err := writeAtomic(s.path, nextText); err != nil {
		return New(), err
	}
	return next, nil
}

func (s *Store) lock() (func(), error) {
	// The lock file and the document share a directory; create it up front so the
	// first writer (reader or desktop) can take the lock before either has
	// written the document.
	if err := os.MkdirAll(filepath.Dir(s.lockPath), 0o755); err != nil {
		return nil, util.Errorf("%w", err)
	}

	lk := flock.New(s.lockPath)
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := lk.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		return nil, util.Errorf("locking reading progress: %w", err)
	}
	if !locked {
		return nil, util.NewError("timed out locking reading progress")
	}
	return func() {
		if unlockErr := lk.Unlock(); unlockErr != nil {
			// The lock is advisory and released when the process exits, so a
			// failed unlock is not worth failing the write over.
			_ = unlockErr
		}
	}, nil
}

func readFile(path string) (string, error) {
	if path == "" {
		return "", util.NewError("reading progress storage is not ready")
	}
	return fsutil.ReadTextFile(path)
}

func writeAtomic(path, text string) error {
	return fsutil.WriteTextFileAtomic(path, ".reading_progress-*.json", text)
}
