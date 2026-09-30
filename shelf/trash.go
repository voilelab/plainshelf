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

// TrashMetaSchemaVersion is the trash.json schema version this build writes.
//
// trash.json is not a cache: it records where each book came from, so rewriting
// it restores the book to the wrong place. It follows book.json's rules — no
// schema_version is read as the current one and persisted lazily, a higher one
// stays listed and readable but refuses any modification.
//
// v2 renamed the origin folder key from "original_layer" to "original_folder".
// It is a hard cut with no dual read: a v1 record's key is not seen, so a book
// trashed by a pre-v2 build restores to the top level of books/. The bump makes
// that visible — a pre-v2 build refuses to modify a v2 record rather than
// rewriting it and dropping the restore path.
const TrashMetaSchemaVersion = 2

// ErrUnsupportedTrashSchemaVersion is returned when an operation would modify a
// trashed book whose on-disk trash.json is newer than this build supports. It is
// separate from ErrUnsupportedBookSchemaVersion so errors.Is can tell which file
// is too new.
var ErrUnsupportedTrashSchemaVersion = util.NewError("trash.json schema version is newer than this build supports")

type TrashedBook struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	Authors        []string      `json:"authors"`
	OriginalPath   string        `json:"original_path,omitempty"`
	OriginalFolder FolderPath    `json:"original_folder,omitempty"`
	DeletedAt      util.JSONTime `json:"deleted_at,omitzero"`

	// NSFW is the shelf's assembled answer for the book while it sits in the
	// trash — see Shelf.IsBookNSFW. It is computed here rather than left to the
	// caller because the folder half of the answer needs OriginalFolder, which
	// only the trash record remembers: trash/ lies outside books/, so the
	// book's path no longer says where the folder rules reach it.
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

	// The folder is the book's position in the tree, owned by its cache entry
	// rather than the book itself; getUpdatedBookFromBookID hands both back from
	// one cache snapshot so the trash record remembers where the book came from.
	book, originalFolder, err := s.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	activePath := book.PackagePath()
	trashPath := path.Join(trashBooksFolder, bookID+bookExtension)

	// A book carried out of the trash by hand keeps its old trash.json, so an
	// active book can still hold one written by a newer build. Refuse before the
	// rename: a guard at writeTrashMeta would already have moved the book into
	// trash/ and would then roll it back, and the rollback is not free — it
	// leaves the user's book somewhere it was not a moment ago if it fails.
	if err := s.ensureTrashMetaWritable(activePath); err != nil {
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

// newTrashedBook assembles one trash listing entry from the book package and
// the trash record beside it. meta may be nil — see readTrashMetaTolerant —
// in which case the book is treated as having come from the top level, which is
// also where restoring it would put it back.
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

// GetTrashedBook returns the listing entry for one trashed book, so a caller
// naming a single book can ask the same questions a listing answers — the
// adult-content mark above among them — without walking the whole trash.
//
// It reports ErrTrashedBookNotFound for an ID that is not in the trash, the
// same as RestoreTrashedBook and DeleteTrashedBook do.
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

	// Restoring moves the book out of trash/ and deletes trash.json, so it is a
	// modification of a record this build may not understand. Refuse here, while
	// nothing on disk has moved yet.
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

	// The original folder may have been deleted while the book sat in trash.
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

	// Deleting the folder deletes trash.json with it, which is as destructive as
	// rewriting it. Refuse before RemoveAll rather than after.
	if err := s.ensureTrashMetaWritable(trashPath); err != nil {
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
	// Stamp unconditionally: the schema version is shelf-managed, so a caller
	// cannot write a version this build does not itself produce.
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

	switch {
	case meta.SchemaVersion > TrashMetaSchemaVersion:
		s.Warn("trash.json schema version is newer than this build supports; the trashed book cannot be modified",
			"path", bookPath,
			"schema_version", meta.SchemaVersion,
			"supported", TrashMetaSchemaVersion)
	default:
		// Missing (pre-v1), zero, or garbage. Normalize in memory only; nothing
		// here rewrites the file.
		meta.SchemaVersion = TrashMetaSchemaVersion
	}

	return meta
}

// trashMetaWritable reports an error when a trashed book's on-disk trash.json
// was written by a newer build, meaning the book must be treated as read-only.
//
// A nil meta — no trash.json, or one hand-edited into something that no longer
// parses — is writable. That file cannot state a version, and refusing on it
// would strand the book in the trash, which is exactly what
// readTrashMetaTolerant exists to prevent.
func trashMetaWritable(meta *trashMeta) error {
	if meta == nil || meta.SchemaVersion <= TrashMetaSchemaVersion {
		return nil
	}
	return util.Errorf("%w: trash.json is schema_version %d, this build writes %d",
		ErrUnsupportedTrashSchemaVersion, meta.SchemaVersion, TrashMetaSchemaVersion)
}

// ensureTrashMetaWritable is trashMetaWritable for a book whose trash.json has
// not been read yet. Call it before any filesystem mutation on that book.
func (s *Shelf) ensureTrashMetaWritable(bookPath string) error {
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
		return nil, util.Errorf("%w", bookpkg.MetadataReadError(metaPath, err))
	}

	return &meta, nil
}

func (s *Shelf) resolveBookPathCollision(folderPath, folderName string) (string, error) {
	baseName := strings.TrimSuffix(folderName, bookExtension)
	if baseName == "" {
		baseName = folderName
	}

	// maxBookPathCollisionAttempts is a practical upper bound for collision resolution in a single folder.
	// If the bound is reached, return an error instead of looping indefinitely.
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

// migrateLegacyTrash moves a shelf written by an older build, which kept the
// trash hidden in ".trash/", onto the visible "trash/" name.
//
// The presence of ".trash/" is the whole detection mechanism: no shelf-level
// manifest is introduced, so the layout gains no file that would have to be
// preserved on backup alongside the disposable contents of app/. That is the
// project-wide policy for shelf-layout changes, not a one-off; the reasoning and
// the test for what counts as a layout change live in
// docs/concepts/data-format-versioning.md, "Shelf layout changes are not
// versioned".
//
// Like the rest of makeStructure this runs without the shelf lock, on the same
// assumption the format documentation states: one PlainShelf version opens a
// shelf at a time.
func (s *Shelf) migrateLegacyTrash(root fsutil.FS) error {
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

// mergeLegacyTrashBooks handles the case where both trash directories exist,
// which happens when a shelf is opened by an older build again after the
// rename. Every book is carried over; nothing under the legacy path is deleted
// unless it is an empty directory left behind by the move.
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

	// Remove, not RemoveAll: it fails on a non-empty directory, so anything the
	// user put under the legacy path by hand is left where they can find it.
	if err := root.Remove(legacyTrashBooksFolder); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Warn("legacy trash directory is not empty; leaving it in place", "path", legacyTrashBooksFolder, "error", err)
		return nil
	}
	if err := root.Remove(legacyTrashFolder); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Warn("legacy trash directory is not empty; leaving it in place", "path", legacyTrashFolder, "error", err)
	}

	return nil
}
