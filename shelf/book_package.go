package shelf

import (
	"errors"
	"io/fs"
	"os"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// ErrNotBookPackage reports a directory that opened, but holds no book.json —
// the caller pointed at a folder that is not a book package.
//
// Distinct from a decode failure: a missing book.json means "wrong folder", a
// malformed one means "this book is damaged", and a reader has to say
// different things about them.
var ErrNotBookPackage = util.NewError("directory is not a book package")

// BookPackage is one book package directory opened on its own, outside any
// shelf: no books/ layout, no app/ directory, no lock, no cache.
//
// A package is self-contained — book.json, sources/, assets/ and the cover all
// live inside it — which is what lets a reader open a folder a user points at
// instead of a whole library.
//
// The package is opened read-only: every mutation on the returned Book is
// refused before it touches a directory the caller may not own. Most report
// fsutil.ErrReadOnly, but a guard that runs earlier answers first — a book.json
// newer than this build is refused as ErrUnsupportedBookSchemaVersion, on a
// package exactly as on a read-only shelf. Test for "was it refused", not for
// one particular error, unless the book's schema version is known.
type BookPackage struct {
	root *os.Root
	book *Book
}

// OpenBookPackage opens the book package in dir for reading.
//
// A nil logger discards the package's own warnings — notably the one about a
// book.json written by a newer build. Pass a logger to see them.
//
// The directory name is not checked for the .bookpkg suffix: what makes a
// folder a book package is that it holds a readable book.json, and a folder
// copied out of a shelf by hand often loses the suffix on the way.
func OpenBookPackage(dir string, logger *logutil.Logger) (*BookPackage, error) {
	if logger == nil {
		discard, err := logutil.NewLogger(&logutil.LogConf{
			LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone},
		})
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		logger = discard
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// The package is its own root, so the book sits at "." and nothing outside
	// the folder the user chose is reachable through this handle.
	book, err := openBook(fsutil.ReadOnly(fsutil.NewRootFS(root)), *logger, ".")
	if err != nil {
		root.Close() //nolint:errcheck // best-effort cleanup; the open error is what matters
		if errors.Is(err, fs.ErrNotExist) {
			return nil, util.Errorf("%w: %s", ErrNotBookPackage, dir)
		}
		return nil, util.Errorf("%w", err)
	}

	return &BookPackage{root: root, book: book}, nil
}

// Book returns the opened book. It stays valid until Close.
func (p *BookPackage) Book() *Book {
	return p.book
}

// Close releases the directory handle the package reads through.
func (p *BookPackage) Close() error {
	if p == nil || p.root == nil {
		return nil
	}

	root := p.root
	p.root = nil
	if err := root.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
