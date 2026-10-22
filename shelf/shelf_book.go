package shelf

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
	"github.com/voilelab/plainshelf/shelf/scancache"
)

type BookListing struct {
	Book *Book

	Folders FolderPath

	CharCount int
}

func (s *Shelf) ListBooks() ([]*Book, error) {
	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var books []*Book
	for _, listing := range listings {
		books = append(books, listing.Book)
	}
	return books, nil
}

func (s *Shelf) ListBooksWithCharCount() ([]BookListing, error) {
	if !s.IsReady() {
		return nil, s.notReady()
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	return s.listBookListingsFromCache(), nil
}

func (s *Shelf) GetBook(bookID string) (*Book, error) {
	if !s.IsReady() {
		return nil, s.notReady()
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, _, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return book, nil
}

func (s *Shelf) NewBook(folders FolderPath, title string) (*Book, error) {
	return s.NewBookWith(folders, title, nil)
}

func (s *Shelf) NewBookWith(folders FolderPath, title string, init func(*Book) error) (*Book, error) {
	if err := s.ValidateFolderPath(folders); err != nil {
		return nil, util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	bookPath, err := createTempDir(root, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer root.RemoveAll(bookPath)

	bookID, err := s.drawUnusedBookID()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	stagedBook, err := bookpkg.Create(root, s.Logger, bookPath, bookID, title)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if init != nil {
		if err := init(stagedBook); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	folderPath := path.Join(booksFolder, path.Join(folders...))

	err = root.MkdirAll(folderPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.addFolderToBookCache(folders)

	folderName := titleToFolderName(title)
	for i := 1; ; i++ {
		finalBookPath := path.Join(folderPath, folderName)
		if _, err := s.dbRoot.Stat(finalBookPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return nil, util.Errorf("%w", err)
		} else {
			folderName = titleToFolderName(fmt.Sprintf("%s-%d", title, i))
		}
	}

	finalBookPath := path.Join(folderPath, folderName)
	err = root.Rename(bookPath, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := bookpkg.Open(s.dbRoot, s.Logger, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.updateBookCacheEntry(folders, finalBookPath, newBook)

	return newBook, nil
}

func (s *Shelf) GetBookListing(bookID string) (BookListing, error) {
	if !s.IsReady() {
		return BookListing{}, s.notReady()
	}

	if err := s.shelfLock.RLock(); err != nil {
		return BookListing{}, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, folders, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return BookListing{}, util.Errorf("%w", err)
	}

	return BookListing{Book: book, Folders: folders}, nil
}

func (s *Shelf) drawUnusedBookID() (string, error) {
	for range MaxBookIDCreationAttempts {
		candidate := newBookID()

		_, _, err := s.getUpdatedBookFromBookID(candidate)
		if errors.Is(err, ErrBookNotFound) {
			inTrash, trashErr := s.isBookIDInTrash(candidate)
			if trashErr != nil {
				return "", util.Errorf("%w", trashErr)
			}
			if !inTrash {
				return candidate, nil
			}
		} else if err != nil {
			return "", util.Errorf("%w", err)
		}
	}

	return "", util.NewError("failed to draw an unused book ID after multiple attempts")
}

func (s *Shelf) DeleteBook(bookID string) error {
	return s.MoveBookToTrash(bookID)
}

func (s *Shelf) MoveBook(bookID string, newFolders FolderPath) (*Book, error) {
	if err := s.ValidateFolderPath(newFolders); err != nil {
		return nil, util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, _, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	newFolderPath := path.Join(booksFolder, path.Join(newFolders...))
	err = root.MkdirAll(newFolderPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.addFolderToBookCache(newFolders)

	newBookPath := path.Join(newFolderPath, path.Base(book.PackagePath()))
	err = root.Rename(book.PackagePath(), newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	movedBook, err := bookpkg.Open(s.dbRoot, s.Logger, newBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	s.updateBookCacheEntry(newFolders, newBookPath, movedBook)

	return movedBook, nil
}

func (s *Shelf) CopyBook(bookID string, targetFolder FolderPath) (*Book, error) {
	if err := s.ValidateFolderPath(targetFolder); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if _, err := s.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	sourceBook, _, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := sourceBook.EnsureWritable(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := s.publishBookCopy(s.dbRoot, sourceBook, targetFolder, false)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return newBook, nil
}

func bookFolderPath(folderPath string) FolderPath {
	return strings.Split(path.Dir(folderPath), "/")[1:]
}

func (s *Shelf) iterateShelfTree(onFolder func(FolderPath) bool, onFolderMeta func(FolderPath), onBook func(*Book) bool) (scanStats, error) {
	skipAll := false

	root := s.walkRoot()
	var unavailable error

	notAShelf := false
	stopIfNotAShelf := func(pth string) {
		if s.remote && pth == booksFolder {
			notAShelf, skipAll = true, true
		}
	}
	stopIfUnavailable := func(err error) bool {
		if errors.Is(err, fsutil.ErrSourceUnavailable) {
			unavailable, skipAll = err, true
			return true
		}
		return false
	}

	walk := s.scanCache.NewWalk()

	var dfsFunc func(string, *scancache.DirChild, bool)

	dfsFunc = func(pth string, child *scancache.DirChild, parentListingReused bool) {
		if skipAll {
			return
		}

		isDir, err := scancache.ChildIsDir(root, pth, child)
		if err != nil {
			if stopIfUnavailable(err) {
				return
			}
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to stat path during book scan", "path", pth, "error", err)
			} else {
				stopIfNotAShelf(pth)
			}
			return
		}

		if onFolderMeta != nil && path.Base(pth) == FolderMetaFile {
			if parent := path.Dir(pth); parent != booksFolder {
				onFolderMeta(strings.Split(parent, "/")[1:])
			}
		}

		if !isDir {
			stopIfNotAShelf(pth)
			return
		}

		folderName := path.Base(pth)
		if s.ignore.IsIgnoredDir(folderName) {
			return
		}

		if strings.HasSuffix(folderName, bookExtension) {
			if onBook == nil {
				return
			}

			book, err := bookpkg.Open(root, s.Logger, pth)
			if err != nil {
				if stopIfUnavailable(err) {
					return
				}
				s.Error("Error opening book", "path", pth, "error", err)
				return
			}

			if !onBook(book) {
				skipAll = true
			}
			return
		}

		if onFolder != nil && !onFolder(strings.Split(pth, "/")[1:]) {
			skipAll = true
			return
		}

		children, childrenTrusted, err := walk.ReadDir(root, pth, parentListingReused)
		if err != nil {
			if stopIfUnavailable(err) {
				return
			}
			if !errors.Is(err, os.ErrNotExist) {
				s.Warn("failed to read directory during book scan", "path", pth, "error", err)
			} else {
				stopIfNotAShelf(pth)
			}
			return
		}

		for i := range children {
			fullPath := path.Join(pth, children[i].Name)
			dfsFunc(fullPath, &children[i], childrenTrusted)
		}
	}

	dfsFunc(booksFolder, nil, true)
	var stats scanStats
	if skipAll {
		stats = walk.Abandon()
	} else {
		stats = walk.Install()
	}
	if unavailable != nil {
		return stats, util.Errorf("%w", unavailable)
	}
	if notAShelf {
		return stats, util.Errorf("%w", ErrNotAShelf)
	}
	return stats, nil
}
