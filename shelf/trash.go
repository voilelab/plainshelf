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
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

const trashMetaFile = "trash.json"
const maxBookPathCollisionAttempts = 10000

var ErrTrashedBookNotFound = util.NewError("trashed book not found")

// TrashMetaSchemaVersion is the trash.json schema version this build writes.
//
// trash.json is not a cache: it records where each book came from, so losing or
// rewriting it restores the book to the wrong place. It follows the same rules
// as book.json. No schema_version predates versioning ("v0"): read as the
// current version, normalized in memory, persisted only on the next write (lazy
// upgrade); opening a shelf never rewrites it. A HIGHER schema_version is still
// listed and readable, but any operation that would modify the trashed book is
// refused before touching the filesystem, so an older build cannot clobber a
// newer one.
//
// v2 renamed the recorded origin folder key from "original_layer" to
// "original_folder" (the layer→folder surface rename). It is a hard cut with no
// dual read: a v1 record's "original_layer" is simply not seen, so a book trashed
// by a pre-v2 build restores to the top level of books/ rather than its old
// folder. The bump makes that visible instead of silent — a pre-v2 build reading
// a v2 record refuses to modify it (ErrUnsupportedTrashSchemaVersion) rather than
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
	trashPath := path.Join(s.trashBooksRoots[0], bookID+bookExtension)

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

	// Read across every trash root (a read-only shelf may have more than one; see
	// resolveTrashBooksRoots). Roots come current path first, and a book folder
	// seen under an earlier root wins, so a name present under both is listed
	// once from the current path rather than twice.
	items := make([]*TrashedBook, 0)
	seen := map[string]struct{}{}
	for _, root := range s.trashBooksRoots {
		entries, err := s.dbRoot.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, util.Errorf("%w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), bookExtension) {
				continue
			}
			if _, dup := seen[entry.Name()]; dup {
				continue
			}
			seen[entry.Name()] = struct{}{}

			bookPath := path.Join(root, entry.Name())
			book, err := bookpkg.Open(s.dbRoot, s.Logger, bookPath)
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
				item.OriginalFolder = append(FolderPath(nil), meta.OriginalFolder...)
			}
			items = append(items, item)
		}
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

	// Across every trash root, current path first, deduped so a book present
	// under both a current and a legacy root is emptied once. See
	// resolveTrashBooksRoots and ListTrashedBooks.
	ids := make([]string, 0)
	seen := map[string]struct{}{}
	for _, root := range s.trashBooksRoots {
		entries, err := s.dbRoot.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, util.Errorf("%w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), bookExtension) {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), bookExtension)
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
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

	if err := validateFolderPath(targetFolders); err != nil {
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

	trashPath := path.Join(s.trashBooksRoots[0], bookID+bookExtension)
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
	trashPath := path.Join(s.trashBooksRoots[0], bookID+bookExtension)
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

	trashPath := path.Join(s.trashBooksRoots[0], bookID+bookExtension)
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

	payload, err := json.MarshalIndent(meta, "", "  ")
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

// resolveTrashBooksRoots decides, once at open time, which directories hold
// this shelf's trashed book packages, returned current path first so a reader
// that walks several lets the current path win on a collision.
//
// A writable shelf has already run migrateLegacyTrash and created
// trashBooksFolder by the time this is called, so its trash is exactly that one
// current directory and no filesystem lookup is needed.
//
// A read-only shelf cannot migrate, so its trash may sit under the current
// path, under the hidden legacyTrashFolder a pre-rename build left, or split
// across BOTH — which is what an older build produces when it reopens an
// already-migrated shelf (see migrateLegacyTrash's merge path). Reading only
// one of them would silently drop every book under the other, the very
// "read-only hides recoverable books" divergence this resolves; so a read-only
// shelf reads across whichever of the two exist.
//
// Directory existence is the whole detection mechanism, the same as
// migrateLegacyTrash; no shelf-level manifest is consulted. This stats
// legacyTrashFolder once even for an already-migrated shelf, because the
// both-exist case can only be seen by looking — the accepted single open-time
// stat, not a guess repeated on every listing.
func (s *Shelf) resolveTrashBooksRoots() []string {
	if !s.readOnly {
		return []string{trashBooksFolder}
	}

	var roots []string
	if info, err := s.dbRoot.Stat(trashFolder); err == nil && info.IsDir() {
		roots = append(roots, trashBooksFolder)
	}
	if info, err := s.dbRoot.Stat(legacyTrashFolder); err == nil && info.IsDir() {
		roots = append(roots, legacyTrashBooksFolder)
	}
	if len(roots) == 0 {
		// Neither layout is present; keep the current path so the listing simply
		// finds it missing and reports an empty trash.
		roots = []string{trashBooksFolder}
	}
	return roots
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
