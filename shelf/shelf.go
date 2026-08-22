package shelf

import (
	"context"
	"errors"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

/*
Layout:
{library}/books/
  {book1-folder}.bookpkg/
  {layer1}/
	{book2-folder}.bookpkg/
	{layer2}/
	  {book2-folder}.bookpkg/
{library}/trash/
  books/
    {book-id}.bookpkg/
      trash.json
{library}/app/
  library.lock
  book-cache-{writer-id}.json
  tmp/
*/

const booksFolder = "books"
const trashFolder = "trash"
const trashBooksFolder = trashFolder + "/" + booksFolder

// Trash used to be hidden as ".trash". It holds books the user deleted, which
// is their data and not disposable runtime state, so it is now a visible
// top-level directory like books/. See migrateLegacyTrash.
const legacyTrashFolder = ".trash"
const legacyTrashBooksFolder = legacyTrashFolder + "/" + booksFolder
const bookExtension = ".bookpkg"
const appFolder = "app"
const appTmpFolder = "tmp"
const libraryLockFile = "library.lock"
const maxPathSegmentLength = 255

var ErrBookNotFound = util.NewError("book not found")
var ErrShelfInitializing = util.NewError("shelf is initializing, please retry shortly")
var ErrShelfLockTimeout = util.NewError("shelf is busy, please retry shortly")

const defaultLockTimeout = 30 * time.Second
const lockRetryDelay = 50 * time.Millisecond

type Shelf struct {
	logutil.Logger

	// dbRoot reads the shelf. It is a ReadFS so that a read path cannot mutate
	// the shelf by accident, and so that a read-only shelf can be represented
	// by a handle that has no write half at all; every mutation narrows it back
	// with writeRoot. See ShelfConf.ReadOnly.
	dbRoot    fsutil.ReadFS
	readOnly  bool
	close     func() error
	shelfLock ShelfLock
	bookCache *bookCache
	ready     atomic.Bool
	readyCh   chan struct{}
	initErr   atomic.Pointer[error] // set if initCache fails; readyCh is still closed

	// scanCacheEnabled is ShelfConf.ScanCache resolved to a decision.
	scanCacheEnabled bool

	// Exported book cache; see shelf_cache_export.go. An empty writer ID
	// disables the export entirely, which is what a bare ShelfConf gets.
	bookCacheWriterID string
	bookCacheInterval time.Duration
	exportMu          sync.Mutex

	// background counts the goroutines a read leaves running; see goBackground.
	backgroundMu sync.Mutex
	background   sync.WaitGroup
	closing      bool
}

type ShelfConf struct {
	Logger  logutil.LogConf `yaml:"logger" json:"logger"`
	LibRoot string          `yaml:"lib_root" json:"lib_root"`

	// ReadOnly opens the shelf without writing to it at all.
	//
	// A read-only shelf is taken exactly as it is found: no directory is
	// created, leftover temp data is not cleared, a legacy .trash/ is not
	// migrated, neither runtime cache under app/ is written, and every mutating
	// operation is refused with fsutil.ErrReadOnly before it touches anything.
	// It also forces lock_mode to "none" and disables the exported book cache,
	// because both of those write to the shelf.
	//
	// Use it for a shelf this process cannot or must not write: a read-only
	// mount or snapshot, a backup image, a share exported read-only. lib_root
	// is never created in this mode - a path that does not open is an error.
	ReadOnly bool `yaml:"read_only" json:"read_only"`

	// for cache

	// Default: 1 minute. This throttles how often a full on-disk scan is performed.
	// Within this interval, refreshes only re-open books already in the cache to
	// update stale metadata.
	// Newly added books may not be discovered until the next full scan.
	// Set to 0s to always perform a full scan on refresh.
	// For SMB mounts, consider increasing this (e.g. "10m") to reduce network I/O.
	ScanInterval string `yaml:"scan_interval" json:"scan_interval"`

	// LockTimeout is the maximum duration to wait when acquiring the shelf lock.
	// On SMB mounts, flock() may behave unreliably; a timeout prevents indefinite hangs.
	// Default: 30s. Set to "0s" to disable the timeout (blocking lock).
	// Only used when lock_mode is "flock".
	LockTimeout string `yaml:"lock_timeout" json:"lock_timeout"`

	// LockMode controls the file locking strategy.
	// "flock" (default): uses OS flock, reliable on local/SMB mounts.
	// "none": disables locking; use when the storage layer cannot support flock
	// (e.g. cloud storage mounted via rclone). Requires the operator to ensure
	// only one PlainShelf instance accesses the shelf at a time.
	LockMode string `yaml:"lock_mode" json:"lock_mode"`

	// BookCheckInterval controls how often per-book staleness checks run (checking whether
	// individual book.json files have changed). Between checks, list operations return from
	// the in-memory cache without any filesystem I/O.
	// Default: same as scan_interval. For SMB mounts, consider setting this to a higher value
	// (e.g. "5m") to reduce network round-trips on list operations.
	BookCheckInterval string `yaml:"book_check_interval" json:"book_check_interval"`

	// ScanCache controls the directory scan cache: the walk remembers each
	// directory's mtime and replaces the next walk's ReadDir with a Stat for
	// every directory that has not changed. The snapshot is kept under app/.
	// See shelf_scan_cache.go and docs/concepts/shelf-cache-and-io.md.
	//
	// nil (the default) is enabled; a non-nil pointer takes its bool value.
	// Turn it off on a mount whose directory mtimes cannot be trusted - some
	// cloud storage gateways do not update them when a child is added, which
	// would keep new books from ever being discovered.
	ScanCache *bool `yaml:"scan_cache" json:"scan_cache"`

	// BookCacheWriterID identifies this installation in the name of the exported
	// book cache it owns, app/book-cache-{id}.json. Several machines can share
	// one shelf without overwriting each other's file.
	//
	// Empty (the default) disables the export. Callers that want it — the server
	// and the desktop app — supply a stable random ID; see shelf_cache_export.go
	// and docs/concepts/shelf-cache-and-io.md.
	BookCacheWriterID string `yaml:"book_cache_writer_id" json:"book_cache_writer_id"`

	// BookCacheInterval throttles how often the exported book cache is
	// refreshed. Default: 1 hour. Set to "0s" to export on every opportunity.
	// Only used when book_cache_writer_id is set.
	//
	// The export is skipped entirely when the shelf content has not changed, so
	// this bounds how quickly a change reaches the file, not how often the file
	// is rewritten.
	BookCacheInterval string `yaml:"book_cache_interval" json:"book_cache_interval"`
}

func NewShelf(conf *ShelfConf) (*Shelf, error) {
	if conf == nil {
		return nil, util.NewError("shelf configuration cannot be nil")
	}

	scanInterval := time.Minute
	if conf.ScanInterval != "" {
		var err error
		scanInterval, err = time.ParseDuration(conf.ScanInterval)
		if err != nil {
			return nil, util.Errorf("invalid scan interval: %w", err)
		}
	}

	lockTimeout := defaultLockTimeout
	if conf.LockTimeout != "" {
		var err error
		lockTimeout, err = time.ParseDuration(conf.LockTimeout)
		if err != nil {
			return nil, util.Errorf("invalid lock timeout: %w", err)
		}
	}

	bookCheckInterval := scanInterval
	if conf.BookCheckInterval != "" {
		var err error
		bookCheckInterval, err = time.ParseDuration(conf.BookCheckInterval)
		if err != nil {
			return nil, util.Errorf("invalid book check interval: %w", err)
		}
	}

	bookCacheInterval := defaultBookCacheInterval
	if conf.BookCacheInterval != "" {
		var err error
		bookCacheInterval, err = time.ParseDuration(conf.BookCacheInterval)
		if err != nil {
			return nil, util.Errorf("invalid book cache interval: %w", err)
		}
	}

	// nil keeps the default (enabled); a non-nil pointer takes its value.
	scanCacheEnabled := true
	if conf.ScanCache != nil {
		scanCacheEnabled = *conf.ScanCache
	}

	// The ID becomes a file name, so anything that could escape app/ or produce
	// an unreadable name is rejected here rather than at write time.
	if err := validateBookCacheWriterID(conf.BookCacheWriterID); err != nil {
		return nil, util.Errorf("%w", err)
	}

	logger, err := logutil.NewLogger(&conf.Logger)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Silently keeping the ID would leave the export enabled on a shelf that
	// must not be written; failing to start over it would make read_only
	// unusable on a config that merely carries the server's default ID.
	bookCacheWriterID := conf.BookCacheWriterID
	if conf.ReadOnly && bookCacheWriterID != "" {
		logger.Warn("ignoring book_cache_writer_id on a read-only shelf: exporting the book cache writes to the shelf", "lib_root", conf.LibRoot)
		bookCacheWriterID = ""
	}

	var rt *os.Root
	rt, err = os.OpenRoot(conf.LibRoot)
	if err != nil {
		// A read-only shelf is never created: the path either already holds a
		// shelf or the caller pointed at the wrong one, and creating it is
		// itself a write.
		if conf.ReadOnly {
			return nil, util.Errorf("%w", err)
		}

		// Auto create the library if it doesn't exist
		if !os.IsNotExist(err) {
			return nil, util.Errorf("%w", err)
		}

		err = os.MkdirAll(conf.LibRoot, 0755)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		rt, err = os.OpenRoot(conf.LibRoot)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	var shelfLock ShelfLock
	switch conf.LockMode {
	case "", "flock":
		shelfLock = newFlockLock(path.Join(conf.LibRoot, appFolder, libraryLockFile), lockTimeout)
	case "none":
		shelfLock = newNoneLock()
	default:
		rt.Close()
		return nil, util.Errorf("unknown lock_mode %q: must be \"flock\" or \"none\"", conf.LockMode)
	}

	// Whatever the config asked for: the lock file is created on first use,
	// which a read-only shelf cannot do, and a shelf that writes nothing has
	// nothing to serialize against another instance. The mode is still
	// validated above so a typo is not hidden by read_only.
	if conf.ReadOnly {
		shelfLock = newNoneLock()
	}

	var dbRoot fsutil.ReadFS = fsutil.NewRootFS(rt)
	if conf.ReadOnly {
		dbRoot = fsutil.ReadOnly(dbRoot)
	}

	s := &Shelf{
		Logger:    *logger,
		dbRoot:    dbRoot,
		readOnly:  conf.ReadOnly,
		close:     rt.Close,
		shelfLock: shelfLock,
		readyCh:   make(chan struct{}),

		// cache
		bookCache:         newBookCache(scanInterval, bookCheckInterval),
		scanCacheEnabled:  scanCacheEnabled,
		bookCacheWriterID: bookCacheWriterID,
		bookCacheInterval: bookCacheInterval,
	}

	err = s.makeStructure()
	if err != nil {
		rt.Close()
		return nil, util.Errorf("%w", err)
	}

	s.Debug("initializing shelf cache in background", "lib_root", conf.LibRoot, "read_only", conf.ReadOnly, "scan_interval", scanInterval, "book_check_interval", bookCheckInterval, "lock_timeout", lockTimeout)
	// Tracked like every other background write: initCache goes on writing the
	// scan cache and the exported book cache after readyCh is closed, so a Close
	// that did not wait would release the shelf lock with a walk still running -
	// which is also what turns those writes into "file already closed" noise
	// today. The cost is that closing mid-scan now waits for that first walk to
	// finish rather than having the filesystem pulled out from under it.
	s.goBackground(func() {
		if err := s.initCache(); err != nil {
			s.Error("failed to initialize shelf cache", "error", err)
		}
	})

	return s, nil
}

func (s *Shelf) makeStructure() error {
	// Every step below writes, and none of them is needed to read a shelf that
	// already exists: books/ and trash/books/ are tolerated missing by the walk
	// and the trash listing, app/tmp/ is only ever used by a mutation, and the
	// legacy trash migration has nothing to migrate for a reader.
	if s.readOnly {
		return nil
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = root.MkdirAll(booksFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Wipe any leftover temp data from a previous run that may have crashed
	// mid-operation (e.g. a partially created book under app/tmp). This folder
	// only ever holds transient in-progress data, so it is safe to clear on startup.
	err = root.RemoveAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = root.MkdirAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Runs before the trash directory is created below, so that a shelf holding
	// only the legacy name can still take the cheap rename path.
	err = s.migrateLegacyTrash(root)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = root.MkdirAll(trashBooksFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// writeRoot narrows the shelf's read handle back to a writable one, reporting
// fsutil.ErrReadOnly when the shelf was opened read-only.
//
// This is the single door between reading and writing the shelf: every mutation
// asks here first, which is what makes ShelfConf.ReadOnly a guarantee the
// compiler enforces rather than a flag each call site has to remember.
func (s *Shelf) writeRoot() (fsutil.FS, error) {
	root, err := fsutil.Writable(s.dbRoot)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return root, nil
}

// ReadOnly reports whether this shelf was opened read-only, in which case every
// mutating operation fails with fsutil.ErrReadOnly.
func (s *Shelf) ReadOnly() bool {
	return s.readOnly
}

func (s *Shelf) initCache() error {
	// Before the first walk, not after: the snapshot from the previous run is
	// what makes this walk - the one the user waits for at startup - cheap.
	s.loadScanCache()

	err := s.scanToBookCache()
	if err != nil {
		wrapped := util.Errorf("%w", err)
		s.initErr.Store(&wrapped)
		close(s.readyCh)
		return wrapped
	}
	s.ready.Store(true)
	close(s.readyCh)
	s.Debug("shelf cache initialized")

	// Export once the shelf is known, so a client that shows up before anything
	// else has read this shelf still finds a usable file.
	if s.bookCacheWriterID != "" {
		if err := s.exportBookCache(false); err != nil {
			s.Warn("failed to export the book cache after the initial scan", "error", err)
		}
	}

	// Offered here as well as at Close so that a process which is killed rather
	// than shut down still leaves a usable snapshot behind. A shelf whose
	// folders did not change since the last run produces the same snapshot, and
	// the digest check turns that into no write at all.
	if err := s.saveScanCache(); err != nil {
		s.Warn("failed to write the directory scan cache after the initial scan", "error", err)
	}

	return nil
}

// IsReady reports whether the initial cache scan completed successfully.
func (s *Shelf) IsReady() bool {
	return s.ready.Load()
}

// InitErr returns the error from the initial cache scan, or nil if it succeeded or is still running.
func (s *Shelf) InitErr() error {
	if errPtr := s.initErr.Load(); errPtr != nil {
		return *errPtr
	}
	return nil
}

// WaitReady blocks until the initial cache scan completes (or fails) or the context is cancelled.
// Returns the init error if initialization failed.
func (s *Shelf) WaitReady(ctx context.Context) error {
	select {
	case <-s.readyCh:
		return s.InitErr()
	case <-ctx.Done():
		return util.Errorf("timed out waiting for shelf to be ready: %w", ctx.Err())
	}
}

// SetScanInterval updates the scan interval on the live shelf without restarting it.
func (s *Shelf) SetScanInterval(scanInterval string) error {
	interval := time.Minute
	if scanInterval != "" {
		var err error
		interval, err = time.ParseDuration(scanInterval)
		if err != nil {
			return util.Errorf("invalid scan interval: %w", err)
		}
	}
	s.bookCache.Lock()
	s.bookCache.scanInterval = interval
	s.bookCache.Unlock()
	return nil
}

// validateBookCacheWriterID rejects anything that would not be safe or legible
// as part of a file name under app/.
func validateBookCacheWriterID(writerID string) error {
	if writerID == "" {
		return nil
	}
	if len(writerID) > 64 {
		return util.NewError("book cache writer id must be at most 64 characters")
	}
	for _, r := range writerID {
		isAllowed := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_'
		if !isAllowed {
			return util.Errorf("book cache writer id may only contain letters, digits, %q and %q", "-", "_")
		}
	}
	return nil
}

// goBackground runs fn in a goroutine that Close waits for, and reports whether
// it started one.
//
// A read schedules work that outlives its call - the per-book staleness check
// and the exported book cache - and both write under app/, the check by taking
// the shelf lock, which creates app/library.lock when missing. Left untracked,
// such a goroutine writes into a shelf whose owner has already closed it and
// released that lock: it can put a file back into a directory the caller is
// deleting, and on a shared shelf it writes without the lock that keeps
// concurrent instances apart.
//
// A closing shelf starts nothing new, which also keeps the counter from being
// raised while Close waits on it.
func (s *Shelf) goBackground(fn func()) bool {
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()

	if s.closing {
		return false
	}

	s.background.Go(fn)
	return true
}

// stopBackground refuses further background work and waits for what is already
// running. It is idempotent, so a second Close is not an error.
func (s *Shelf) stopBackground() {
	s.backgroundMu.Lock()
	s.closing = true
	s.backgroundMu.Unlock()

	s.background.Wait()
}

// Close releases any resources held by the Shelf instance.
func (s *Shelf) Close() error {
	// Before anything else, and before the lock is released below: a straggler
	// would otherwise write to a shelf this process has stopped holding.
	s.stopBackground()

	errs := []error{}

	// Flush before the lock goes away: this is the one write guaranteed to
	// happen, and it keeps the exported cache useful for a desktop app opened,
	// used and closed inside a single interval.
	//
	// Offered unconditionally, because the only trustworthy answer to "did
	// anything change" is to compare the content — a book edited in place
	// through *Book leaves no other trace here. exportBookCache does that
	// comparison, writes nothing when the shelf is unchanged, and reads the
	// in-memory cache rather than disk, so a quiet shutdown costs nothing beyond
	// hashing what is already in memory.
	if s.bookCacheWriterID != "" {
		if err := s.exportBookCache(false); err != nil {
			// Shutdown must not fail over a rebuildable file.
			s.Warn("failed to export the book cache while closing", "error", err)
		}
	}

	// Same reasoning as the export above, and equally not worth failing a
	// shutdown over: the snapshot is rebuildable runtime state.
	if err := s.saveScanCache(); err != nil {
		s.Warn("failed to write the directory scan cache while closing", "error", err)
	}

	if err := s.shelfLock.Close(); err != nil {
		errs = append(errs, err)
	}

	if s.close != nil {
		err := s.close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	err := s.Logger.Close()
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return util.Errorf("%w", errors.Join(errs...))
	}

	return nil
}
