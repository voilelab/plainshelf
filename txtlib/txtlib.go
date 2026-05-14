package txtlib

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"go.rtnl.ai/x/slugify"
)

/*
Layout:
{library}/books/
  l/
    lotm-7k3x/
  s/
    sicp-v2m8/
  b/
    book-a82m/
{library}/app/
  tmp/
*/

const booksFolder = "books"
const bookExtension = ".novl"
const appFolder = "app"
const appTmpFolder = "tmp"

var ErrBookNotFound = util.NewError("book not found")

type Lib struct {
	dbRoot   fsutil.FS
	readonly bool
	close    func() error
}

// OpenLocalLib initializes a new Lib instance with the given library root path.
func OpenLocalLib(libRoot string) (*Lib, error) {
	var rt *os.Root
	rt, err := os.OpenRoot(libRoot)
	if err != nil {
		// Auto create the library if it doesn't exist
		if os.IsNotExist(err) {
			err = os.MkdirAll(libRoot, 0755)
			if err != nil {
				return nil, util.Errorf("%w", err)
			}
			rt, err = os.OpenRoot(libRoot)
			if err != nil {
				return nil, util.Errorf("%w", err)
			}
		}
	}

	txtLib := &Lib{dbRoot: fsutil.NewRootFS(rt), readonly: false, close: rt.Close}
	err = txtLib.makeStructure()
	if err != nil {
		rt.Close()
		return nil, util.Errorf("%w", err)
	}

	return txtLib, nil
}

// OpenLib initializes a new Lib instance with the given fsutil.FS as the library root.
func OpenLib(root fsutil.FS, readonly bool) (*Lib, error) {
	txtLib := &Lib{dbRoot: root, readonly: readonly}

	if !readonly {
		err := txtLib.makeStructure()
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	return txtLib, nil
}

func (t *Lib) makeStructure() error {
	// create the directory structure for the library
	err := t.dbRoot.MkdirAll(booksFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = t.dbRoot.MkdirAll(path.Join(appFolder, appTmpFolder))
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// Close releases any resources held by the Lib instance.
func (t *Lib) Close() error {
	if t.close == nil {
		return nil
	}

	err := t.close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// ListBooks returns a list of all books in the library.
// Books are sorted by their ID in ascending order.
func (t *Lib) ListBooks() ([]*Book, error) {
	var books []*Book

	err := t.iterateBooks(nil, func(b *Book) bool {
		books = append(books, b)
		return true
	})

	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sort.Slice(books, func(i, j int) bool {
		return books[i].ID() < books[j].ID()
	})

	return books, nil
}

// GetBook returns the details of a specific book by its ID.
func (t *Lib) GetBook(bookID string) (*Book, error) {
	book, err := t.getBook(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return book, nil
}

// NewBook creates a new book with the given ID and title, and returns the created Book instance.
// It is an atomic operation that ensures the book is fully created before it becomes visible in the library.
func (t *Lib) NewBook(layers Layers, title string) (*Book, error) {
	bookPath, err := createTempDir(t.dbRoot, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer t.dbRoot.RemoveAll(bookPath)

	// Generate a unique book ID based on the layers and title
	baseBookID := generateBookID(layers, title)
	bookID := baseBookID
	for i := 1; ; i++ {
		_, err := t.getBook(bookID)
		if err != nil {
			if errors.Is(err, ErrBookNotFound) {
				break
			}
			return nil, util.Errorf("%w", err)
		} else {
			bookID = fmt.Sprintf("%s-%d", baseBookID, i)
		}
	}

	_, err = createBook(t.dbRoot, bookPath, bookID, title)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	layerPath := path.Join(booksFolder, path.Join(layers...))

	err = t.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	folderName := titleToFolderName(title)
	for i := 1; ; i++ {
		finalBookPath := path.Join(layerPath, folderName)
		if _, err := t.dbRoot.Stat(finalBookPath); err != nil {
			// TBD: handle error other than not exist
			break
		} else {
			folderName = titleToFolderName(fmt.Sprintf("%s-%d", title, i))
		}
	}

	finalBookPath := path.Join(layerPath, folderName)
	err = t.dbRoot.Rename(bookPath, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := openBook(t.dbRoot, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook.setLayers(layers)

	return newBook, nil
}

// DeleteBook removes a book from the library by its ID.
func (t *Lib) DeleteBook(bookID string) error {
	book, err := t.getBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = t.dbRoot.RemoveAll(book.FolderPath())
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// GetAllLayers returns a sorted list of all unique layers present in the library.
func (t *Lib) GetAllLayers() ([]Layers, error) {
	var layers []Layers
	seen := make(map[string]bool)
	err := t.iterateLayers(func(ls Layers) bool {
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
func (t *Lib) GetBooksByLayer(layers Layers) ([]*Book, error) {
	var books []*Book

	err := t.iterateBooks(layers, func(b *Book) bool {
		books = append(books, b)
		return true
	})

	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sort.Slice(books, func(i, j int) bool {
		return books[i].ID() < books[j].ID()
	})

	return books, nil
}

// MoveBook moves a book to new layers and returns the updated Book instance.
func (t *Lib) MoveBook(bookID string, newLayers Layers) (*Book, error) {
	book, err := t.getBook(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newLayerPath := path.Join(booksFolder, path.Join(newLayers...))
	err = t.dbRoot.MkdirAll(newLayerPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBookPath := path.Join(newLayerPath, path.Base(book.FolderPath()))
	err = t.dbRoot.Rename(book.FolderPath(), newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook, err := openBook(t.dbRoot, newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook.setLayers(newLayers)
	return movedBook, nil
}

func (t *Lib) getBook(bookID string) (*Book, error) {
	var book *Book
	err := t.iterateBooks(nil, func(b *Book) bool {
		if b.ID() == bookID {
			book = b
			return false
		}
		return true
	})

	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if book == nil {
		return nil, util.Errorf("%w: %s", ErrBookNotFound, bookID)
	}

	return book, nil
}

// iterateBooks iterates over all books under the specified layers and applies the provided function to each book.
// If the function returns false, the iteration will stop.
func (t *Lib) iterateBooks(rLayers Layers, fn func(*Book) bool) error {
	visitFolder := path.Join(booksFolder, path.Join(rLayers...))

	skipAll := false

	var dfsFunc func(string)

	dfsFunc = func(pth string) {
		if skipAll {
			return
		}

		stat, err := t.dbRoot.Stat(pth)
		if err != nil {
			return
		}

		if !stat.IsDir() {
			return
		}

		folderName := path.Base(pth)
		if strings.HasSuffix(folderName, bookExtension) {
			book, err := openBook(t.dbRoot, pth)
			if err != nil {
				log.Println("Error opening book:", err)
				return
			}

			layers := strings.Split(path.Dir(pth), string(os.PathSeparator))[1:]
			book.setLayers(layers)

			if !fn(book) {
				skipAll = true
			}
			return
		}

		entries, err := t.dbRoot.ReadDir(pth)
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
func (t *Lib) iterateLayers(fn func(Layers) bool) error {
	skipAll := false

	var dfsFunc func(string)

	dfsFunc = func(pth string) {
		if skipAll {
			return
		}

		stat, err := t.dbRoot.Stat(pth)
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

		entries, err := t.dbRoot.ReadDir(pth)
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
func (t *Lib) NewLayer(layer Layers) error {
	for _, l := range layer {
		if strings.Contains(l, bookExtension) {
			return util.Errorf("invalid layer name: %s", l)
		}
	}

	layerPath := path.Join(booksFolder, path.Join(layer...))
	err := t.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// DeleteLayer removes a layer from the library. It checks if the layer is empty (i.e., contains no books) before deleting it. If the layer is not empty, it returns an error.
func (t *Lib) DeleteLayer(layer Layers) error {
	layerPath := path.Join(booksFolder, path.Join(layer...))

	entries, err := t.dbRoot.ReadDir(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if len(entries) > 0 {
		return util.Errorf("cannot delete non-empty layer")
	}

	err = t.dbRoot.RemoveAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
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
