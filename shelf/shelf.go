package shelf

import (
	"context"
	"errors"
	"os"
	"path"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
	"github.com/voilelab/plainshelf/shelf/scancache"
)

const booksFolder = "books"
const trashFolder = "trash"
const trashBooksFolder = trashFolder + "/" + booksFolder

const legacyTrashFolder = ".trash"
const legacyTrashBooksFolder = legacyTrashFolder + "/" + booksFolder
const bookExtension = ".bookpkg"
const appFolder = "app"
const appTmpFolder = "tmp"
const libraryLockFile = "library.lock"

var ErrBookNotFound = util.NewError("book not found")
var ErrShelfInitializing = util.NewError("shelf is initializing, please retry shortly")
var ErrShelfLockTimeout = util.NewError("shelf is busy, please retry shortly")

const defaultLockTimeout = 30 * time.Second
const lockRetryDelay = 50 * time.Millisecond

type Shelf struct {
	logutil.Logger

	dbRoot    fsutil.ReadFS
	readOnly  bool
	close     func() error
	shelfLock ShelfLock
	bookCache *bookCache
	ready     atomic.Bool
	readyCh   chan struct{}
	initErr   atomic.Pointer[error]

	awaitingSource      atomic.Bool
	initRetryMu         sync.Mutex
	lastInitTryUnixNano atomic.Int64

	scanCache *scancache.Cache

	ignore shelfutil.IgnoreRules

	nsfw shelfutil.NSFWRules

	bookCacheWriterID string
	bookCacheInterval time.Duration
	exportMu          sync.Mutex

	backgroundMu sync.Mutex
	background   sync.WaitGroup
	closing      bool

	remote bool

	remoteRoot fsutil.ReadFS
	offline    *offlineStore

	folderMetaWarnings sync.Map
}

type ShelfConf struct {
	Logger  logutil.LogConf `yaml:"logger" json:"logger"`
	LibRoot string          `yaml:"lib_root" json:"lib_root"`

	Source *SourceConf `yaml:"source" json:"source,omitzero"`

	ReadOnly bool `yaml:"read_only" json:"read_only"`

	ScanInterval string `yaml:"scan_interval" json:"scan_interval"`

	LockTimeout string `yaml:"lock_timeout" json:"lock_timeout"`

	LockMode string `yaml:"lock_mode" json:"lock_mode"`

	BookCheckInterval string `yaml:"book_check_interval" json:"book_check_interval"`

	ScanCache *bool `yaml:"scan_cache" json:"scan_cache"`

	BookCacheWriterID string `yaml:"book_cache_writer_id" json:"book_cache_writer_id"`

	BookCacheInterval string `yaml:"book_cache_interval" json:"book_cache_interval"`
}

type shelfSettings struct {
	logger            *logutil.Logger
	scanInterval      time.Duration
	lockTimeout       time.Duration
	bookCheckInterval time.Duration
	bookCacheInterval time.Duration
	scanCacheEnabled  bool
	bookCacheWriterID string
}

func parseShelfSettings(conf *ShelfConf) (*shelfSettings, error) {
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

	scanCacheEnabled := true
	if conf.ScanCache != nil {
		scanCacheEnabled = *conf.ScanCache
	}

	if err := validateBookCacheWriterID(conf.BookCacheWriterID); err != nil {
		return nil, util.Errorf("%w", err)
	}

	logger, err := logutil.NewLogger(&conf.Logger)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	bookCacheWriterID := conf.BookCacheWriterID
	if conf.ReadOnly && bookCacheWriterID != "" {
		logger.Warn("ignoring book_cache_writer_id on a read-only shelf: exporting the book cache writes to the shelf", "lib_root", conf.LibRoot)
		bookCacheWriterID = ""
	}

	return &shelfSettings{
		logger:            logger,
		scanInterval:      scanInterval,
		lockTimeout:       lockTimeout,
		bookCheckInterval: bookCheckInterval,
		bookCacheInterval: bookCacheInterval,
		scanCacheEnabled:  scanCacheEnabled,
		bookCacheWriterID: bookCacheWriterID,
	}, nil
}

func NewShelf(conf *ShelfConf) (*Shelf, error) {
	if conf == nil {
		return nil, util.NewError("shelf configuration cannot be nil")
	}

	src, err := conf.ResolveSource()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if src.IsVirtual() {
		return nil, util.Errorf("source type %q is not a local directory and cannot be opened here", src.Type)
	}
	localConf := *conf
	localConf.LibRoot, localConf.Source = src.Path, nil
	conf = &localConf

	settings, err := parseShelfSettings(conf)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var rt *os.Root
	rt, err = os.OpenRoot(conf.LibRoot)
	if err != nil {
		if conf.ReadOnly {
			return nil, util.Errorf("%w", err)
		}

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
		shelfLock = newFlockLock(path.Join(conf.LibRoot, appFolder, libraryLockFile), settings.lockTimeout)
	case "none":
		shelfLock = newNoneLock()
	default:
		rt.Close()
		return nil, util.Errorf("unknown lock_mode %q: must be \"flock\" or \"none\"", conf.LockMode)
	}

	if conf.ReadOnly {
		shelfLock = newNoneLock()
	}

	var dbRoot fsutil.ReadFS = fsutil.NewRootFS(rt)
	if conf.ReadOnly {
		dbRoot = fsutil.ReadOnly(dbRoot)
	}

	return startShelf(conf, settings, dbRoot, rt.Close, shelfLock, false, nil)
}

func NewShelfFromFS(conf *ShelfConf, root fsutil.ReadFS, offlineDir string) (*Shelf, error) {
	if conf == nil {
		return nil, util.NewError("shelf configuration cannot be nil")
	}
	if root == nil {
		return nil, util.NewError("shelf root cannot be nil")
	}

	remoteConf := *conf
	remoteConf.ReadOnly = true
	remoteConf.ScanCache = new(false)

	settings, err := parseShelfSettings(&remoteConf)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var offline *offlineStore
	dbRoot := root
	var closeOffline func() error
	if offlineDir != "" {
		offline, err = openOfflineStore(offlineDir, *settings.logger)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		routed := offlineFS{remote: root, store: offline}
		dbRoot = routed
		if _, ok := root.(fsutil.ZipOpener); ok {
			dbRoot = offlineZipFS{routed}
		}
		closeOffline = offline.close
	}

	s, err := startShelf(&remoteConf, settings, fsutil.ReadOnly(dbRoot), closeOffline, newNoneLock(), true, func(s *Shelf) {
		s.remoteRoot = root
		s.offline = offline
	})
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return s, nil
}

func startShelf(conf *ShelfConf, settings *shelfSettings, dbRoot fsutil.ReadFS, closeRoot func() error, shelfLock ShelfLock, remote bool, configure func(*Shelf)) (*Shelf, error) {
	logger := settings.logger

	rules := loadShelfRules(dbRoot, *logger)

	s := &Shelf{
		Logger:    *logger,
		dbRoot:    dbRoot,
		readOnly:  conf.ReadOnly,
		close:     closeRoot,
		remote:    remote,
		shelfLock: shelfLock,
		readyCh:   make(chan struct{}),

		ignore: rules.ignore,
		nsfw:   rules.nsfw,

		bookCache:         newBookCache(settings.scanInterval, settings.bookCheckInterval),
		scanCache:         newScanCache(dbRoot, settings.scanCacheEnabled, *logger),
		bookCacheWriterID: settings.bookCacheWriterID,
		bookCacheInterval: settings.bookCacheInterval,
	}
	if configure != nil {
		configure(s)
	}

	if err := s.makeStructure(); err != nil {
		if closeRoot != nil {
			_ = closeRoot()
		}
		return nil, util.Errorf("%w", err)
	}

	s.Debug("initializing shelf cache in background", "lib_root", conf.LibRoot, "read_only", conf.ReadOnly, "remote", remote, "scan_interval", settings.scanInterval, "book_check_interval", settings.bookCheckInterval, "lock_timeout", settings.lockTimeout)
	s.goBackground(func() {
		if err := s.initCache(); err != nil {
			s.Error("failed to initialize shelf cache", "error", err)
		}
	})

	return s, nil
}

func (s *Shelf) makeStructure() error {
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

	err = root.RemoveAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = root.MkdirAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = s.ensureTrashFolder(root)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (s *Shelf) writeRoot() (fsutil.FS, error) {
	root, err := fsutil.Writable(s.dbRoot)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return root, nil
}

func (s *Shelf) ensureTrashFolder(root fsutil.FS) error {
	if err := s.migrateLegacyTrash(root); err != nil {
		return util.Errorf("%w", err)
	}
	if err := root.MkdirAll(trashBooksFolder); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (s *Shelf) exportEnabled() bool {
	return s.bookCacheWriterID != ""
}

func (s *Shelf) ReadOnly() bool {
	return s.readOnly
}

func (s *Shelf) LocalFiles() (fsutil.ReadFS, bool) {
	if s.remote {
		return nil, false
	}
	return fsutil.ReadOnly(s.dbRoot), true
}

func (s *Shelf) initCache() error {
	var err error
	if s.remote {
		err = s.refreshRemoteBookCache(s.openWithoutExport)
	} else {
		err = s.scanToBookCache()
	}
	if err != nil {
		wrapped := util.Errorf("%w", err)
		s.initErr.Store(&wrapped)
		if s.remote && isAwaitingSource(err) {
			s.lastInitTryUnixNano.Store(time.Now().UnixNano())
			s.awaitingSource.Store(true)
		}
		close(s.readyCh)
		return wrapped
	}
	s.ready.Store(true)
	close(s.readyCh)
	s.Debug("shelf cache initialized")

	if s.exportEnabled() {
		if err := s.exportBookCache(false); err != nil {
			s.Warn("failed to export the book cache after the initial scan", "error", err)
		}
	}

	if err := s.scanCache.Save(); err != nil {
		s.Warn("failed to write the directory scan cache after the initial scan", "error", err)
	}

	return nil
}

var initRetryInterval = 10 * time.Second

func isAwaitingSource(err error) bool {
	return errors.Is(err, fsutil.ErrSourceUnavailable) || errors.Is(err, ErrNotAShelf)
}

func (s *Shelf) notReady() error {
	if s.awaitingSource.Load() {
		s.retryInitSoon()
		if err := s.InitErr(); err != nil {
			return util.Errorf("%w", err)
		}
	}
	return util.Errorf("%w", ErrShelfInitializing)
}

func (s *Shelf) retryInitSoon() {
	if time.Since(time.Unix(0, s.lastInitTryUnixNano.Load())) < initRetryInterval {
		return
	}
	s.goBackground(func() {
		if !s.initRetryMu.TryLock() {
			return
		}
		defer s.initRetryMu.Unlock()
		if !s.IsReady() {
			_ = s.retryInit()
		}
	})
}

func (s *Shelf) retryInit() error {
	s.lastInitTryUnixNano.Store(time.Now().UnixNano())
	err := s.refreshRemoteBookCache(s.openWithoutExport)
	if err != nil {
		wrapped := util.Errorf("%w", err)
		s.initErr.Store(&wrapped)
		if !isAwaitingSource(err) {
			s.awaitingSource.Store(false)
		}
		return wrapped
	}
	s.initErr.Store(nil)
	s.ready.Store(true)
	s.awaitingSource.Store(false)
	s.Info("shelf source is reachable again; the shelf is listed")
	return nil
}

func (s *Shelf) IsReady() bool {
	return s.ready.Load()
}

func (s *Shelf) InitErr() error {
	if errPtr := s.initErr.Load(); errPtr != nil {
		return *errPtr
	}
	return nil
}

func (s *Shelf) WaitReady(ctx context.Context) error {
	select {
	case <-s.readyCh:
		return s.InitErr()
	case <-ctx.Done():
		return util.Errorf("timed out waiting for shelf to be ready: %w", ctx.Err())
	}
}

func (s *Shelf) SetIntervals(scanInterval, bookCheckInterval string) error {
	interval := time.Minute
	if scanInterval != "" {
		var err error
		interval, err = time.ParseDuration(scanInterval)
		if err != nil {
			return util.Errorf("invalid scan interval: %w", err)
		}
	}
	checkInterval := interval
	if bookCheckInterval != "" {
		var err error
		checkInterval, err = time.ParseDuration(bookCheckInterval)
		if err != nil {
			return util.Errorf("invalid book check interval: %w", err)
		}
	}
	s.bookCache.Lock()
	defer s.bookCache.Unlock()
	s.bookCache.scanInterval = interval
	s.bookCache.bookCheckInterval = checkInterval
	return nil
}

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

func (s *Shelf) goBackground(fn func()) bool {
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()

	if s.closing {
		return false
	}

	s.background.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				s.Error("background work panicked", "panic", r, "stack", string(debug.Stack()))
			}
		}()
		fn()
	})
	return true
}

func (s *Shelf) stopBackground() {
	s.backgroundMu.Lock()
	s.closing = true
	s.backgroundMu.Unlock()

	s.background.Wait()
}

func (s *Shelf) Close() error {
	s.stopBackground()

	errs := []error{}

	if s.exportEnabled() {
		if err := s.exportBookCache(false); err != nil {
			s.Warn("failed to export the book cache while closing", "error", err)
		}
	}

	if err := s.scanCache.Save(); err != nil {
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
