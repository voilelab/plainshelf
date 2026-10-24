package readerapi

import (
	"sync"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const ShelfID = "book"

type Library struct {
	mu     sync.RWMutex
	logger *logutil.Logger
	pkg    *bookpkg.BookPackage
}

func NewLibrary(logger *logutil.Logger) *Library {
	return &Library{logger: logger}
}

func (l *Library) Open(dir string) (string, error) {
	pkg, err := bookpkg.OpenBookPackage(dir, l.logger)
	if err != nil {
		return "", util.Errorf("%w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_ = l.closeLocked()
	l.pkg = pkg

	return pkg.Book().ID(), nil
}

func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.closeLocked()
}

func (l *Library) closeLocked() error {
	pkg := l.pkg
	l.pkg = nil

	if pkg == nil {
		return nil
	}
	if err := pkg.Close(); err != nil {
		l.warn("failed to close a book package", "error", err)
		return util.Errorf("%w", err)
	}
	return nil
}

func (l *Library) BookID() string {
	bookID := ""
	l.withBook(func(book *bookpkg.Book) { bookID = book.ID() })
	return bookID
}

func (l *Library) withBook(read func(*bookpkg.Book)) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.pkg == nil {
		return false
	}

	read(l.pkg.Book())
	return true
}

func (l *Library) warn(msg string, args ...any) {
	if l.logger != nil {
		l.logger.Warn(msg, args...)
	}
}
