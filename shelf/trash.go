package shelf

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const trashMetaFile = "trash.json"
const maxBookPathCollisionAttempts = 10000

var ErrTrashedBookNotFound = util.NewError("trashed book not found")

type TrashedBook struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Authors       []string      `json:"authors"`
	OriginalPath  string        `json:"original_path,omitempty"`
	OriginalLayer Layers        `json:"original_layer,omitempty"`
	DeletedAt     util.JSONTime `json:"deleted_at,omitzero"`
}

type trashMeta struct {
	DeletedAt     util.JSONTime `json:"deleted_at,omitzero"`
	OriginalPath  string        `json:"original_path,omitempty"`
	OriginalLayer Layers        `json:"original_layer,omitempty"`
	DeleteReason  string        `json:"delete_reason,omitempty"`
}

func (s *Shelf) MoveBookToTrash(bookID string) error {
	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	activePath := book.FolderPath()
	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)

	if _, err := s.dbRoot.Stat(trashPath); err == nil {
		return util.Errorf("book %q already exists in trash", bookID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if err := s.dbRoot.Rename(activePath, trashPath); err != nil {
		return util.Errorf("%w", err)
	}

	meta := trashMeta{
		DeletedAt:     util.JSONTime(time.Now()),
		OriginalPath:  activePath,
		OriginalLayer: append(Layers(nil), book.Layers()...),
		DeleteReason:  "user",
	}
	if err := s.writeTrashMeta(trashPath, &meta); err != nil {
		_ = s.dbRoot.Rename(trashPath, activePath)
		return util.Errorf("%w", err)
	}

	s.deleteBookCacheEntry(bookID)
	return nil
}

func (s *Shelf) ListTrashedBooks() ([]*TrashedBook, error) {
	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	entries, err := s.dbRoot.ReadDir(trashBooksFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}

	items := make([]*TrashedBook, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), bookExtension) {
			continue
		}

		bookPath := path.Join(trashBooksFolder, entry.Name())
		book, err := openBook(s.dbRoot, s.Logger, bookPath)
		if err != nil {
			s.Warn("failed to open trashed book, skipping", "path", bookPath, "error", err)
			continue
		}

		meta := s.readTrashMetaTolerant(bookPath)

		item := &TrashedBook{
			ID:      book.ID(),
			Title:   book.Title(),
			Authors: append([]string(nil), book.GetMeta().Authors...),
		}
		if meta != nil {
			item.DeletedAt = meta.DeletedAt
			item.OriginalPath = meta.OriginalPath
			item.OriginalLayer = append(Layers(nil), meta.OriginalLayer...)
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].DeletedAt != items[j].DeletedAt {
			return time.Time(items[i].DeletedAt).After(time.Time(items[j].DeletedAt))
		}
		return items[i].ID < items[j].ID
	})

	return items, nil
}

// ListTrashedBookIDs returns the ID of every book directory under the trash.
//
// Unlike ListTrashedBooks it neither opens the books nor reads their trash
// metadata, so it also reports entries that cannot be opened. Emptying the
// trash must see those, otherwise it would leave behind directories the UI
// never showed.
func (s *Shelf) ListTrashedBookIDs() ([]string, error) {
	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	entries, err := s.dbRoot.ReadDir(trashBooksFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), bookExtension) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), bookExtension))
	}

	sort.Strings(ids)
	return ids, nil
}

func (s *Shelf) RestoreTrashedBook(bookID string) error {
	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	trashPath, book, meta, err := s.findTrashedBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	targetLayers := Layers(nil)
	targetFolder := path.Base(trashPath)
	if meta != nil {
		targetLayers = append(Layers(nil), meta.OriginalLayer...)
		if base := path.Base(meta.OriginalPath); strings.HasSuffix(base, bookExtension) {
			targetFolder = base
		}
	}

	if err := validateLayers(targetLayers); err != nil {
		targetLayers = nil
	}

	targetLayerPath := path.Join(booksFolder, path.Join(targetLayers...))
	if err := s.dbRoot.MkdirAll(targetLayerPath); err != nil {
		return util.Errorf("%w", err)
	}

	// The original layer may have been deleted while the book sat in trash.
	s.addLayersToBookCache(targetLayers)

	targetPath, err := s.resolveBookPathCollision(targetLayerPath, targetFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.dbRoot.Rename(trashPath, targetPath); err != nil {
		return util.Errorf("%w", err)
	}
	_ = s.dbRoot.Remove(path.Join(targetPath, trashMetaFile))

	restoredBook, err := openBook(s.dbRoot, s.Logger, targetPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	restoredBook.setLayers(targetLayers)
	s.updateBookCacheEntry(restoredBook.Layers(), targetPath, restoredBook)

	if restoredBook.ID() != book.ID() {
		return util.Errorf("restored book id mismatch")
	}

	return nil
}

func (s *Shelf) DeleteTrashedBook(bookID string) error {
	if err := validateBookID(bookID); err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)
	if _, err := s.dbRoot.Stat(trashPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("%w", ErrTrashedBookNotFound)
		}
		return util.Errorf("%w", err)
	}

	if err := s.dbRoot.RemoveAll(trashPath); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (s *Shelf) isBookIDInTrash(bookID string) (bool, error) {
	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)
	_, err := s.dbRoot.Stat(trashPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, util.Errorf("%w", err)
}

func (s *Shelf) findTrashedBook(bookID string) (string, *Book, *trashMeta, error) {
	if err := validateBookID(bookID); err != nil {
		return "", nil, nil, util.Errorf("%w", err)
	}

	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)
	book, err := openBook(s.dbRoot, s.Logger, trashPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil, util.Errorf("%w", ErrTrashedBookNotFound)
		}
		return "", nil, nil, util.Errorf("%w", err)
	}

	return trashPath, book, s.readTrashMetaTolerant(trashPath), nil
}

func (s *Shelf) writeTrashMeta(bookPath string, meta *trashMeta) error {
	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteFileAtomic(s.dbRoot, path.Join(bookPath, trashMetaFile), payload); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

// readTrashMetaTolerant reads a trashed book's metadata and reports nil when it
// is absent or unusable.
//
// The shelf is hand-editable, so trash.json can be truncated or edited into
// something that no longer parses. That must not hide the book from the trash
// or refuse to restore it: the file only records where the book came from, and
// a book without it is simply restored to the top level of books/.
func (s *Shelf) readTrashMetaTolerant(bookPath string) *trashMeta {
	meta, err := s.readTrashMeta(bookPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.Warn("failed to read trash metadata, treating the book as having none", "path", bookPath, "error", err)
		}
		return nil
	}
	return meta
}

func (s *Shelf) readTrashMeta(bookPath string) (*trashMeta, error) {
	fp, err := s.dbRoot.Open(path.Join(bookPath, trashMetaFile))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer fp.Close()

	var meta trashMeta
	if err := json.NewDecoder(fp).Decode(&meta); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &meta, nil
}

func (s *Shelf) resolveBookPathCollision(layerPath, folderName string) (string, error) {
	baseName := strings.TrimSuffix(folderName, bookExtension)
	if baseName == "" {
		baseName = folderName
	}

	// maxBookPathCollisionAttempts is a practical upper bound for collision resolution in a single layer.
	// If the bound is reached, return an error instead of looping indefinitely.
	for i := range maxBookPathCollisionAttempts {
		candidateFolder := folderName
		if i > 0 {
			candidateFolder = baseName + "-" + strconv.Itoa(i) + bookExtension
		}
		candidatePath := path.Join(layerPath, candidateFolder)
		_, err := s.dbRoot.Stat(candidatePath)
		if errors.Is(err, os.ErrNotExist) {
			return candidatePath, nil
		}
		if err != nil {
			return "", util.Errorf("%w", err)
		}
	}

	return "", util.Errorf("failed to resolve unique book path for %q", folderName)
}

// migrateLegacyTrash moves a shelf written by an older build, which kept the
// trash hidden in ".trash/", onto the visible "trash/" name.
//
// The presence of ".trash/" is the whole detection mechanism: no shelf-level
// manifest is introduced, so the layout gains no file that would have to be
// preserved on backup alongside the disposable contents of app/.
//
// Like the rest of makeStructure this runs without the shelf lock, on the same
// assumption the format documentation states: one PlainShelf version opens a
// shelf at a time.
func (s *Shelf) migrateLegacyTrash() error {
	legacyInfo, err := s.dbRoot.Stat(legacyTrashFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return util.Errorf("%w", err)
	}
	if !legacyInfo.IsDir() {
		// Not something this migration wrote; leave the user's file alone.
		return nil
	}

	_, err = s.dbRoot.Stat(trashFolder)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := s.dbRoot.Rename(legacyTrashFolder, trashFolder); err != nil {
			return util.Errorf("%w", err)
		}
		s.Info("renamed legacy trash directory", "from", legacyTrashFolder, "to", trashFolder)
		return nil
	case err != nil:
		return util.Errorf("%w", err)
	}

	return s.mergeLegacyTrashBooks()
}

// mergeLegacyTrashBooks handles the case where both trash directories exist,
// which happens when a shelf is opened by an older build again after the
// rename. Every book is carried over; nothing under the legacy path is deleted
// unless it is an empty directory left behind by the move.
func (s *Shelf) mergeLegacyTrashBooks() error {
	entries, err := s.dbRoot.ReadDir(legacyTrashBooksFolder)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if len(entries) > 0 {
		if err := s.dbRoot.MkdirAll(trashBooksFolder); err != nil {
			return util.Errorf("%w", err)
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), bookExtension) {
			continue
		}

		src := path.Join(legacyTrashBooksFolder, entry.Name())
		dst, err := s.resolveBookPathCollision(trashBooksFolder, entry.Name())
		if err != nil {
			return util.Errorf("%w", err)
		}
		if err := s.dbRoot.Rename(src, dst); err != nil {
			return util.Errorf("%w", err)
		}
		if dst != path.Join(trashBooksFolder, entry.Name()) {
			s.Warn("legacy trashed book collided with an existing one; kept under a new folder name", "from", src, "to", dst)
		} else {
			s.Info("moved legacy trashed book", "from", src, "to", dst)
		}
	}

	// Remove, not RemoveAll: it fails on a non-empty directory, so anything the
	// user put under the legacy path by hand is left where they can find it.
	if err := s.dbRoot.Remove(legacyTrashBooksFolder); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Warn("legacy trash directory is not empty; leaving it in place", "path", legacyTrashBooksFolder, "error", err)
		return nil
	}
	if err := s.dbRoot.Remove(legacyTrashFolder); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Warn("legacy trash directory is not empty; leaving it in place", "path", legacyTrashFolder, "error", err)
	}

	return nil
}
