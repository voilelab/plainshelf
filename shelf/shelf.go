package shelf

import (
	"context"
	"errors"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/gofrs/flock"
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
	dbRoot      fsutil.FS
	readonly    bool
	close       func() error
	localLock   *flock.Flock
	lockTimeout time.Duration
	bookCache   *bookCache
	ready       atomic.Bool
	readyCh     chan struct{}
	initErr     atomic.Pointer[error] // set if initCache fails; readyCh is still closed
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
	LockTimeout string `yaml:"lock_timeout" json:"lock_timeout"`
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

	s := &Shelf{
		Logger:      *logger,
		dbRoot:      fsutil.NewRootFS(rt),
		readonly:    false,
		close:       rt.Close,
		localLock:   flock.New(path.Join(conf.LibRoot, appFolder, libraryLockFile)),
		lockTimeout: lockTimeout,
		readyCh:     make(chan struct{}),

		// cache
		bookCache: newBookCache(scanInterval),
	}

	err = s.makeStructure()
	if err != nil {
		rt.Close()
		return nil, util.Errorf("%w", err)
	}

	s.Debug("initializing shelf cache in background", "lib_root", conf.LibRoot, "scan_interval", scanInterval, "lock_timeout", lockTimeout)
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

func (s *Shelf) lock() error {
	if s.readonly || s.localLock == nil {
		return nil
	}

	if s.lockTimeout == 0 {
		return s.localLock.Lock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.lockTimeout)
	defer cancel()
	locked, err := s.localLock.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		return util.Errorf("%w: %w", ErrShelfLockTimeout, err)
	}
	if !locked {
		return util.Errorf("%w: write lock timed out, another instance may hold it", ErrShelfLockTimeout)
	}
	return nil
}

func (s *Shelf) rlock() error {
	if s.readonly || s.localLock == nil {
		return nil
	}

	if s.lockTimeout == 0 {
		return s.localLock.RLock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.lockTimeout)
	defer cancel()
	locked, err := s.localLock.TryRLockContext(ctx, lockRetryDelay)
	if err != nil {
		return util.Errorf("%w: %w", ErrShelfLockTimeout, err)
	}
	if !locked {
		return util.Errorf("%w: read lock timed out, another instance may hold it", ErrShelfLockTimeout)
	}
	return nil
}

func (s *Shelf) unlock() error {
	if s.readonly {
		return nil
	}

	if s.localLock == nil {
		return nil
	}

	err := s.localLock.Unlock()
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// Close releases any resources held by the Shelf instance.
func (s *Shelf) Close() error {
	errs := []error{}
	if s.localLock != nil {
		err := s.localLock.Close()
		if err != nil {
			errs = append(errs, err)
		}
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
