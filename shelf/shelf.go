package shelf

import (
	"context"
	"errors"
	"os"
	"path"
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
{library}/app/
  library.lock
  tmp/
*/

const booksFolder = "books"
const trashFolder = ".trash"
const trashBooksFolder = trashFolder + "/" + booksFolder
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
	dbRoot    fsutil.FS
	close     func() error
	shelfLock ShelfLock
	bookCache *bookCache
	ready     atomic.Bool
	readyCh   chan struct{}
	initErr   atomic.Pointer[error] // set if initCache fails; readyCh is still closed
}

type ShelfConf struct {
	Logger  logutil.LogConf `yaml:"logger" json:"logger"`
	LibRoot string          `yaml:"lib_root" json:"lib_root"`

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

	logger, err := logutil.NewLogger(&conf.Logger)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var rt *os.Root
	rt, err = os.OpenRoot(conf.LibRoot)
	if err != nil {
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

	s := &Shelf{
		Logger:    *logger,
		dbRoot:    fsutil.NewRootFS(rt),
		close:     rt.Close,
		shelfLock: shelfLock,
		readyCh:   make(chan struct{}),

		// cache
		bookCache: newBookCache(scanInterval, bookCheckInterval),
	}

	err = s.makeStructure()
	if err != nil {
		rt.Close()
		return nil, util.Errorf("%w", err)
	}

	s.Debug("initializing shelf cache in background", "lib_root", conf.LibRoot, "scan_interval", scanInterval, "book_check_interval", bookCheckInterval, "lock_timeout", lockTimeout)
	go func() {
		if err := s.initCache(); err != nil {
			s.Error("failed to initialize shelf cache", "error", err)
		}
	}()

	return s, nil
}

func (s *Shelf) makeStructure() error {
	// create the directory structure for the library
	err := s.dbRoot.MkdirAll(booksFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Wipe any leftover temp data from a previous run that may have crashed
	// mid-operation (e.g. a partially created book under app/tmp). This folder
	// only ever holds transient in-progress data, so it is safe to clear on startup.
	err = s.dbRoot.RemoveAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = s.dbRoot.MkdirAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = s.dbRoot.MkdirAll(trashBooksFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (s *Shelf) initCache() error {
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

// Close releases any resources held by the Shelf instance.
func (s *Shelf) Close() error {
	errs := []error{}
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
