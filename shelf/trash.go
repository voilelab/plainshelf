package shelf

import (
	"cmp"
	"encoding/json/v2"
	"errors"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const trashMetaFile = "trash.json"
const maxBookPathCollisionAttempts = 10000

var ErrTrashedBookNotFound = util.NewError("trashed book not found")

const TrashMetaSchemaVersion = 2

var ErrUnsupportedTrashSchemaVersion = util.NewError("trash.json schema version is newer than this build supports")

type TrashedBook struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	Authors        []string      `json:"authors"`
	OriginalPath   string        `json:"original_path,omitempty"`
	OriginalFolder FolderPath    `json:"original_folder,omitempty"`
	DeletedAt      util.JSONTime `json:"deleted_at,omitzero"`

	NSFW bool `json:"nsfw,omitempty"`
}

type trashMeta struct {
	// SchemaVersion is the on-disk format version of trash.json. It is
	// shelf-managed: writeTrashMeta stamps it, and any value a caller supplies
	// is ignored. Declared first so it marshals as the first key, matching
	// book.json.
	SchemaVersion  int           `json:"schema_version"`
	DeletedAt      util.JSONTime `json:"deleted_at,omitzero"`
	OriginalPath   string        `json:"original_path,omitempty"`
	OriginalFolder FolderPath    `json:"original_folder,omitempty"`
	DeleteReason   string        `json:"delete_reason,omitempty"`
}

func (s *Shelf) MoveBookToTrash(bookID string) error {
	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	book, originalFolder, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	activePath := book.PackagePath()
	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)

	if err := s.refuseIfTrashMetaTooNew(activePath); err != nil {
		return util.Errorf("%w", err)
	}

	if _, err := s.dbRoot.Stat(trashPath); err == nil {
		return util.Errorf("book %q already exists in trash", bookID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if err := root.Rename(activePath, trashPath); err != nil {
		return util.Errorf("%w", err)
	}

	meta := trashMeta{
		DeletedAt:      util.JSONTime(time.Now()),
		OriginalPath:   activePath,
		OriginalFolder: append(FolderPath(nil), originalFolder...),
		DeleteReason:   "user",
	}
	if err := s.writeTrashMeta(root, trashPath, &meta); err != nil {
		_ = root.Rename(trashPath, activePath)
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
		book, err := bookpkg.Open(s.dbRoot, s.Logger, bookPath)
		if err != nil {
			s.Warn("failed to open trashed book, skipping", "path", bookPath, "error", err)
			continue
		}

		items = append(items, s.newTrashedBook(book, s.readTrashMetaTolerant(bookPath)))
	}

	slices.SortFunc(items, func(a, b *TrashedBook) int {
		return cmp.Or(
			time.Time(b.DeletedAt).Compare(time.Time(a.DeletedAt)),
			cmp.Compare(a.ID, b.ID),
		)
	})

	return items, nil
}

func (s *Shelf) newTrashedBook(book *Book, meta *trashMeta) *TrashedBook {
	item := &TrashedBook{
		ID:      book.ID(),
		Title:   book.Title(),
		Authors: append([]string(nil), book.GetMeta().Authors...),
	}
	if meta != nil {
		item.DeletedAt = meta.DeletedAt
		item.OriginalPath = meta.OriginalPath
		item.OriginalFolder = append(FolderPath(nil), meta.OriginalFolder...)
	}
	item.NSFW = s.IsBookNSFW(item.OriginalFolder, book.GetMeta())
	return item
}

func (s *Shelf) GetTrashedBook(bookID string) (*TrashedBook, error) {
	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	_, book, meta, err := s.findTrashedBook(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return s.newTrashedBook(book, meta), nil
}

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

	slices.Sort(ids)
	return ids, nil
}

func (s *Shelf) RestoreTrashedBook(bookID string) error {
	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	trashPath, book, meta, err := s.findTrashedBook(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := trashMetaWritable(meta); err != nil {
		return util.Errorf("%w", err)
	}

	targetFolders := FolderPath(nil)
	targetFolder := path.Base(trashPath)
	if meta != nil {
		targetFolders = append(FolderPath(nil), meta.OriginalFolder...)
		if base := path.Base(meta.OriginalPath); strings.HasSuffix(base, bookExtension) {
			targetFolder = base
		}
	}

	if err := s.ValidateFolderPath(targetFolders); err != nil {
		targetFolders = nil
	}

	targetFolderPath := path.Join(booksFolder, path.Join(targetFolders...))
	if err := root.MkdirAll(targetFolderPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.addFolderToBookCache(targetFolders)

	targetPath, err := s.resolveBookPathCollision(targetFolderPath, targetFolder)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := root.Rename(trashPath, targetPath); err != nil {
		return util.Errorf("%w", err)
	}
	_ = root.Remove(path.Join(targetPath, trashMetaFile))

	restoredBook, err := bookpkg.Open(s.dbRoot, s.Logger, targetPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	s.updateBookCacheEntry(targetFolders, targetPath, restoredBook)

	if restoredBook.ID() != book.ID() {
		return util.Errorf("restored book id mismatch")
	}

	return nil
}

func (s *Shelf) DeleteTrashedBook(bookID string) error {
	if err := validateBookID(bookID); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
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

	if err := s.refuseIfTrashMetaTooNew(trashPath); err != nil {
		return util.Errorf("%w", err)
	}

	if err := root.RemoveAll(trashPath); err != nil {
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
	book, err := bookpkg.Open(s.dbRoot, s.Logger, trashPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil, util.Errorf("%w", ErrTrashedBookNotFound)
		}
		return "", nil, nil, util.Errorf("%w", err)
	}

	return trashPath, book, s.readTrashMetaTolerant(trashPath), nil
}

func (s *Shelf) writeTrashMeta(root fsutil.FS, bookPath string, meta *trashMeta) error {
	meta.SchemaVersion = TrashMetaSchemaVersion

	payload, err := json.Marshal(meta, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteFileAtomic(root, path.Join(bookPath, trashMetaFile), payload); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}

func (s *Shelf) readTrashMetaTolerant(bookPath string) *trashMeta {
	meta, err := s.readTrashMeta(bookPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.Warn("failed to read trash metadata, treating the book as having none", "path", bookPath, "error", err)
		}
		return nil
	}

	switch {
	case meta.SchemaVersion > TrashMetaSchemaVersion:
		s.Warn("trash.json schema version is newer than this build supports; the trashed book cannot be modified",
			"path", bookPath,
			"schema_version", meta.SchemaVersion,
			"supported", TrashMetaSchemaVersion)
	default:
		meta.SchemaVersion = TrashMetaSchemaVersion
	}

	return meta
}

func trashMetaWritable(meta *trashMeta) error {
	if meta == nil || meta.SchemaVersion <= TrashMetaSchemaVersion {
		return nil
	}
	return util.Errorf("%w: trash.json is schema_version %d, this build writes %d",
		ErrUnsupportedTrashSchemaVersion, meta.SchemaVersion, TrashMetaSchemaVersion)
}

func (s *Shelf) refuseIfTrashMetaTooNew(bookPath string) error {
	return trashMetaWritable(s.readTrashMetaTolerant(bookPath))
}

func (s *Shelf) readTrashMeta(bookPath string) (*trashMeta, error) {
	metaPath := path.Join(bookPath, trashMetaFile)
	fp, err := s.dbRoot.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer fp.Close()

	var meta trashMeta
	if err := json.UnmarshalRead(fp, &meta); err != nil {
		return nil, util.Errorf("%w", bookpkg.NameMetadataDecodeError(metaPath, err))
	}

	return &meta, nil
}

func (s *Shelf) resolveBookPathCollision(folderPath, folderName string) (string, error) {
	baseName := strings.TrimSuffix(folderName, bookExtension)
	if baseName == "" {
		baseName = folderName
	}

	for i := range maxBookPathCollisionAttempts {
		candidateFolder := folderName
		if i > 0 {
			candidateFolder = baseName + "-" + strconv.Itoa(i) + bookExtension
		}
		candidatePath := path.Join(folderPath, candidateFolder)
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

func (s *Shelf) migrateLegacyTrash(root fsutil.FS) error {
	legacyInfo, err := s.dbRoot.Stat(legacyTrashFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return util.Errorf("%w", err)
	}
	if !legacyInfo.IsDir() {
		return nil
	}

	_, err = s.dbRoot.Stat(trashFolder)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := root.Rename(legacyTrashFolder, trashFolder); err != nil {
			return util.Errorf("%w", err)
		}
		s.Info("renamed legacy trash directory", "from", legacyTrashFolder, "to", trashFolder)
		return nil
	case err != nil:
		return util.Errorf("%w", err)
	}

	return s.mergeLegacyTrashBooks(root)
}

func (s *Shelf) mergeLegacyTrashBooks(root fsutil.FS) error {
	entries, err := s.dbRoot.ReadDir(legacyTrashBooksFolder)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if len(entries) > 0 {
		if err := root.MkdirAll(trashBooksFolder); err != nil {
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
		if err := root.Rename(src, dst); err != nil {
			return util.Errorf("%w", err)
		}
		if dst != path.Join(trashBooksFolder, entry.Name()) {
			s.Warn("legacy trashed book collided with an existing one; kept under a new folder name", "from", src, "to", dst)
		} else {
			s.Info("moved legacy trashed book", "from", src, "to", dst)
		}
	}

	if err := root.Remove(legacyTrashBooksFolder); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Warn("legacy trash directory is not empty; leaving it in place", "path", legacyTrashBooksFolder, "error", err)
		return nil
	}
	if err := root.Remove(legacyTrashFolder); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Warn("legacy trash directory is not empty; leaving it in place", "path", legacyTrashFolder, "error", err)
	}

	return nil
}
