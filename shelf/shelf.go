package shelf

import (
	"errors"
	"os"
	"path"
	"time"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

/*
Layout:
{library}/books/
  {book1-folder}.novl/
  {layer1}/
	{book2-folder}.novl/
	{layer2}/
	  {book2-folder}.novl/
{library}/app/
  library.lock
  tmp/
*/

const booksFolder = "books"
const trashFolder = ".trash"
const trashBooksFolder = trashFolder + "/" + booksFolder
const bookExtension = ".novl"
const appFolder = "app"
const appTmpFolder = "tmp"
const libraryLockFile = "library.lock"
const maxPathSegmentLength = 255

var ErrBookNotFound = util.NewError("book not found")

type Shelf struct {
	logutil.Logger
	dbRoot    fsutil.FS
	readonly  bool
	close     func() error
	localLock *flock.Flock
	bookCache *bookCache
}

type ShelfConf struct {
	Logger  logutil.LogConf `yaml:"logger"`
	LibRoot string          `yaml:"lib_root"`

	// for cache

	// Default: 1 minute. This throttles how often a full on-disk scan is performed.
	// Within this interval, refreshes only re-open books already in the cache to
	// update stale metadata.
	// Newly added books may not be discovered until the next full scan.
	// Set to 0s to always perform a full scan on refresh.
	ScanInterval string `yaml:"scan_interval"`
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

	shelf := &Shelf{
		Logger:    *logger,
		dbRoot:    fsutil.NewRootFS(rt),
		readonly:  false,
		close:     rt.Close,
		localLock: flock.New(path.Join(conf.LibRoot, appFolder, libraryLockFile)),

		// cache
		bookCache: newBookCache(scanInterval),
	}

	err = shelf.makeStructure()
	if err != nil {
		rt.Close()
		return nil, util.Errorf("%w", err)
	}

	err = shelf.initCache()
	if err != nil {
		rt.Close()
		return nil, util.Errorf("%w", err)
	}

	return shelf, nil
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
		return util.Errorf("%w", err)
	}

	return nil
}

func (s *Shelf) lock() error {
	if s.readonly {
		return nil
	}

	if s.localLock == nil {
		return nil
	}

	err := s.localLock.Lock()
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (s *Shelf) rlock() error {
	if s.readonly {
		return nil
	}

	if s.localLock == nil {
		return nil
	}

	err := s.localLock.RLock()
	if err != nil {
		return util.Errorf("%w", err)
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
