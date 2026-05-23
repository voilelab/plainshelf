package shelf

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofrs/flock"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"go.rtnl.ai/x/slugify"
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

	// Default: 1 minute. This is to prevent too frequent full scans of the book cache,
	// which can be expensive if there are many books.
	// If the cache is marked as dirty but the last full scan was performed
	// recently (within this interval), we will skip the full scan and only
	// refresh the books that are currently in the cache.
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

// ListBooks returns a list of all books in the library.
// Books are sorted by their ID in ascending order.
func (s *Shelf) ListBooks() ([]*Book, error) {
	s.rlock()
	defer s.unlock()

	err := s.refreshBookCacheIfNeeded(false)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return s.listBooksFromCache(), nil
}

// GetBook returns the details of a specific book by its ID.
func (s *Shelf) GetBook(bookID string) (*Book, error) {
	s.rlock()
	defer s.unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return book, nil
}

// NewBook creates a new book with the given ID and title, and returns the created Book instance.
// It is an atomic operation that ensures the book is fully created before it becomes visible in the library.
func (s *Shelf) NewBook(layers Layers, title string) (*Book, error) {
	if err := validateLayers(layers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.lock()
	defer s.unlock()

	bookPath, err := createTempDir(s.dbRoot, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.dbRoot.RemoveAll(bookPath)

	// Generate a unique book ID based on the layers and title
	// TBD: Use UUID
	baseBookID := generateBookID(layers, title)
	bookID := baseBookID
	for i := 1; ; i++ {
		_, err := s.getUpdatedBookFromBookID(bookID)
		if err != nil {
			if errors.Is(err, ErrBookNotFound) {
				break
			}
			return nil, util.Errorf("%w", err)
		} else {
			bookID = fmt.Sprintf("%s-%d", baseBookID, i)
		}
	}

	_, err = createBook(s.dbRoot, s.Logger, bookPath, bookID, title)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	layerPath := path.Join(booksFolder, path.Join(layers...))

	err = s.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	folderName := titleToFolderName(title)
	for i := 1; ; i++ {
		finalBookPath := path.Join(layerPath, folderName)
		if _, err := s.dbRoot.Stat(finalBookPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return nil, util.Errorf("%w", err)
		} else {
			folderName = titleToFolderName(fmt.Sprintf("%s-%d", title, i))
		}
	}

	finalBookPath := path.Join(layerPath, folderName)
	err = s.dbRoot.Rename(bookPath, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := openBook(s.dbRoot, s.Logger, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook.setLayers(layers)

	s.updateBookCacheEntry(layers, finalBookPath, newBook)

	return newBook, nil
}

// DeleteBook removes a book from the library by its ID.
func (s *Shelf) DeleteBook(bookID string) error {
	s.lock()
	defer s.unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = s.dbRoot.RemoveAll(book.FolderPath())
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.deleteBookCacheEntry(bookID)

	return nil
}

// GetAllLayers returns a sorted list of all unique layers present in the library.
func (s *Shelf) GetAllLayers() ([]Layers, error) {
	s.rlock()
	defer s.unlock()

	var layers []Layers
	seen := make(map[string]bool)
	err := s.iterateLayers(func(ls Layers) bool {
		key := strings.Join(ls, "/")
		if !seen[key] {
			layers = append(layers, ls)
			seen[key] = true
		}
		return true
	})

	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sort.Slice(layers, func(i, j int) bool {
		return strings.Join(layers[i], "/") < strings.Join(layers[j], "/")
	})

	return layers, nil
}

// GetBooksByLayer returns a list of books that belong to the specified layers.
func (s *Shelf) GetBooksByLayer(layers Layers) ([]*Book, error) {
	if err := validateLayers(layers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.rlock()
	defer s.unlock()

	err := s.refreshBookCacheIfNeeded(false)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var books []*Book

	for _, book := range s.listBooksFromCache() {
		if book.Layers().Equal(layers) {
			books = append(books, book)
		}
	}

	return books, nil
}

// MoveBook moves a book to new layers and returns the updated Book instance.
func (s *Shelf) MoveBook(bookID string, newLayers Layers) (*Book, error) {
	if err := validateLayers(newLayers); err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.lock()
	defer s.unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newLayerPath := path.Join(booksFolder, path.Join(newLayers...))
	err = s.dbRoot.MkdirAll(newLayerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBookPath := path.Join(newLayerPath, path.Base(book.FolderPath()))
	err = s.dbRoot.Rename(book.FolderPath(), newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook, err := openBook(s.dbRoot, s.Logger, newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook.setLayers(newLayers)

	s.updateBookCacheEntry(newLayers, newBookPath, movedBook)

	return movedBook, nil
}

// iterateBooks iterates over all books under the specified layers and applies the provided function to each book.
// If the function returns false, the iteration will stop.
func (s *Shelf) iterateBooks(rLayers Layers, fn func(*Book) bool) error {
	visitFolder := path.Join(booksFolder, path.Join(rLayers...))

	skipAll := false

	var dfsFunc func(string)

	dfsFunc = func(pth string) {
		if skipAll {
			return
		}

		stat, err := s.dbRoot.Stat(pth)
		if err != nil {
			return
		}

		if !stat.IsDir() {
			return
		}

		folderName := path.Base(pth)
		if strings.HasSuffix(folderName, bookExtension) {
			book, err := openBook(s.dbRoot, s.Logger, pth)
			if err != nil {
				s.Error("Error opening book", "path", pth, "error", err)
				return
			}

			layers := strings.Split(path.Dir(pth), string(os.PathSeparator))[1:]
			book.setLayers(layers)

			if !fn(book) {
				skipAll = true
			}
			return
		}

		entries, err := s.dbRoot.ReadDir(pth)
		if err != nil {
			return
		}

		for _, entry := range entries {
			fullPath := path.Join(pth, entry.Name())
			dfsFunc(fullPath)
		}
	}

	dfsFunc(visitFolder)
	return nil
}

// iterateLayers iterates over all unique layers in the library and applies the provided function to each layer.
// If the function returns false, the iteration will stop.
func (s *Shelf) iterateLayers(fn func(Layers) bool) error {
	skipAll := false

	var dfsFunc func(string)

	dfsFunc = func(pth string) {
		if skipAll {
			return
		}

		stat, err := s.dbRoot.Stat(pth)
		if err != nil {
			return
		}

		if !stat.IsDir() {
			return
		}

		folderName := path.Base(pth)
		if strings.HasSuffix(folderName, bookExtension) {
			return
		}

		layers := strings.Split(pth, string(os.PathSeparator))[1:]
		if !fn(layers) {
			skipAll = true
			return
		}

		entries, err := s.dbRoot.ReadDir(pth)
		if err != nil {
			return
		}

		for _, entry := range entries {
			fullPath := path.Join(pth, entry.Name())
			dfsFunc(fullPath)
		}
	}

	dfsFunc(booksFolder)
	return nil
}

// NewLayer creates a new layer in the library. It validates the layer name to ensure it does not contain invalid characters and then creates the necessary directory structure for the layer.
func (s *Shelf) NewLayer(layer Layers) error {
	if err := validateLayers(layer); err != nil {
		return util.Errorf("%w", err)
	}

	s.lock()
	defer s.unlock()

	layerPath := path.Join(booksFolder, path.Join(layer...))
	err := s.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// DeleteLayer removes a layer from the library. It checks if the layer is empty (i.e., contains no books) before deleting it. If the layer is not empty, it returns an error.
func (s *Shelf) DeleteLayer(layer Layers) error {
	if err := validateLayers(layer); err != nil {
		return util.Errorf("%w", err)
	}

	s.lock()
	defer s.unlock()

	layerPath := path.Join(booksFolder, path.Join(layer...))

	entries, err := s.dbRoot.ReadDir(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if len(entries) > 0 {
		return util.Errorf("cannot delete non-empty layer")
	}

	err = s.dbRoot.RemoveAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func validateLayers(layers Layers) error {
	for _, layer := range layers {
		if err := validatePathSegment(layer); err != nil {
			return util.Errorf("invalid layer name %q: %w", layer, err)
		}
		if strings.Contains(layer, bookExtension) {
			return util.Errorf("invalid layer name %q: must not contain %q", layer, bookExtension)
		}
	}
	return nil
}

func validateSourceID(sourceID string) error {
	if err := validatePathSegment(sourceID); err != nil {
		return util.Errorf("invalid source id %q: %w", sourceID, err)
	}
	return nil
}

func validatePathSegment(segment string) error {
	if segment == "" {
		return util.NewError("path segment cannot be empty")
	}
	if segment == "." || segment == ".." {
		return util.NewError("path segment cannot be . or ..")
	}
	if strings.ContainsAny(segment, `/\`) {
		return util.NewError("path segment cannot contain path separators")
	}
	if !utf8.ValidString(segment) {
		return util.NewError("path segment must be valid UTF-8")
	}
	if len(segment) > maxPathSegmentLength {
		return util.Errorf("path segment exceeds %d bytes", maxPathSegmentLength)
	}
	return nil
}

func generateBookID(layers Layers, title string) string {
	cont := strings.Join(layers, "-") + "-" + title
	md5Hash := md5.Sum([]byte(cont))
	hash := fmt.Sprintf("%x", md5Hash)
	return hash[:8] // Use the first 8 characters of the hash as the book ID
}

func titleToFolderName(title string) string {
	// Replace spaces with dashes and remove special characters for folder naming
	folderName := strings.ReplaceAll(title, " ", "-")
	return slugify.Slugify(folderName) + bookExtension
}
