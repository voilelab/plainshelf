package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"maps"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

/*
folder.json is a folder's own metadata, kept in the folder directory so that it
moves with the directory: a rename, a move, or an `mv` outside PlainShelf all
carry it along. See docs/concepts/data-format-versioning.md#folderjson-schema-v1.

It is user data, not a cache, and it is the first shelf file written member by
member (jsonopt.Object): a write changes the members it names and keeps every
other one, so a field a newer build added survives an older build's edit.

Reading never fails a listing. A missing, unreadable or unsupported file reads
as "no metadata", and only a write refuses it — rewriting a file this build
cannot read would lose whatever is in it.
*/

// FolderMetaFile is the metadata file's name inside a folder directory. It is
// reserved: no folder may be named this (see validateNewFolderName).
const FolderMetaFile = "folder.json"

// FolderMetaSchemaVersion is the folder.json version this build reads and
// writes. A file without schema_version reads as this version.
const FolderMetaSchemaVersion = 1

// maxFolderMetaBytes bounds how large a folder.json this build reads. The file
// is a few lines; a larger one is not something PlainShelf wrote.
const maxFolderMetaBytes = 64 << 10

const (
	folderMetaSchemaVersionKey = "schema_version"
	folderMetaIconKey          = "icon"
	folderMetaSortKeyKey       = "sort_key"
)

// ErrReservedFolderName refuses a folder named after FolderMetaFile, which would
// collide with its parent's metadata.
var ErrReservedFolderName = util.Errorf("%w: %q is reserved for folder metadata", ErrInvalidFolder, FolderMetaFile)

// ErrInvalidFolderIcon refuses an icon name that is not in FolderIcons.
var ErrInvalidFolderIcon = util.NewError("folder icon is not one this build offers")

// ErrFolderMetaNotWritable refuses a write to a folder.json this build cannot
// read safely: malformed, not an object, or from a newer schema version.
var ErrFolderMetaNotWritable = util.NewError("folder.json cannot be updated by this build")

// ErrFolderNotFound refuses a metadata write to a folder that does not exist.
var ErrFolderNotFound = util.NewError("folder not found")

// ErrFolderMetaConflict refuses a write whose caller read a different
// folder.json than the one on disk now.
var ErrFolderMetaConflict = util.NewError("folder.json changed since it was read")

// folderIcons are the icon names a folder may be given: Tabler icon names, so
// the frontend can render each one from @iconify-json/tabler. Adding a name is
// compatible; removing one leaves files that name it reading as no icon.
var folderIcons = []string{
	"alien", "atom", "bolt", "book", "bookmark", "books", "briefcase", "camera",
	"code", "coffee", "compass", "crown", "device-gamepad-2", "diamond", "flame",
	"flask", "flower", "folder", "ghost", "heart", "leaf", "map", "mood-smile",
	"moon", "movie", "music", "notebook", "palette", "paw", "planet", "puzzle",
	"robot", "rocket", "school", "shield", "skull", "star", "sun", "sword", "world",
}

// FolderIcons returns the icon names this build offers, sorted.
func FolderIcons() []string {
	return slices.Clone(folderIcons)
}

// IsFolderIcon reports whether name is one of FolderIcons.
func IsFolderIcon(name string) bool {
	_, found := slices.BinarySearch(folderIcons, name)
	return found
}

// FolderMeta is one folder's folder.json as this build reads it.
type FolderMeta struct {
	// SortKey orders siblings when HasSortKey is true; zero and negatives are valid.
	SortKey    float64
	HasSortKey bool

	// Icon is the icon member as written, "" when absent, of the wrong type, or
	// unreadable. It may name an icon this build does not offer; see
	// DisplayIcon.
	Icon string

	// ReadOnly reports that a write would be refused (ErrFolderMetaNotWritable):
	// the file does not parse, is not an object, or carries a schema_version
	// this build does not know. Whether the shelf itself is writable is a
	// separate question.
	ReadOnly bool

	// Version identifies the file content read, "" when there is no file. A
	// write passes it back to detect a change made in between.
	Version string
}

// DisplayIcon is the icon to show: Icon when this build offers it, "" (the
// default folder icon) otherwise.
func (m FolderMeta) DisplayIcon() string {
	if IsFolderIcon(m.Icon) {
		return m.Icon
	}
	return ""
}

// ReadFolderMeta reads folder's folder.json from disk now. Every problem with
// the file reads as no metadata and is logged once per distinct problem, never
// as an error: a listing must not fail on a hand-edited file. The error is for
// an invalid path or a shelf that cannot be read at all. A listing reads the
// cached copy instead; see ListFolderMeta.
func (s *Shelf) ReadFolderMeta(folder FolderPath) (FolderMeta, error) {
	if len(folder) == 0 {
		return FolderMeta{}, nil
	}
	if err := s.ValidateFolderPath(folder); err != nil {
		return FolderMeta{}, util.Errorf("%w", err)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return FolderMeta{}, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	return s.loadFolderMeta(s.dbRoot, folder)
}

// loadFolderMeta is ReadFolderMeta without the checks and the lock, for a
// caller that already holds it. The only error is an unavailable source.
func (s *Shelf) loadFolderMeta(root fsutil.ReadFS, folder FolderPath) (FolderMeta, error) {
	metaPath := folderMetaPath(folder)
	data, err := readFolderMetaFile(root, metaPath)
	if err != nil {
		if errors.Is(err, fsutil.ErrSourceUnavailable) {
			return FolderMeta{}, util.Errorf("%w", err)
		}
		s.warnFolderMeta(metaPath, "failed to read folder metadata, showing the folder without it", err)
		return FolderMeta{ReadOnly: true}, nil
	}
	meta, err := folderMetaFromData(data)
	if err != nil {
		s.warnFolderMeta(metaPath, "folder metadata cannot be read, showing the folder without it", err)
		return meta, nil
	}
	s.folderMetaWarnings.Delete(metaPath)
	return meta, nil
}

// folderMetaFromData reads a folder.json's bytes, nil for no file. The error
// says why the file is unusable; the FolderMeta returned with it is the
// fallback to show.
func folderMetaFromData(data []byte) (FolderMeta, error) {
	if data == nil {
		return FolderMeta{}, nil
	}

	meta := FolderMeta{Version: folderMetaVersion(data)}
	obj, err := parseFolderMeta(data)
	if err != nil {
		meta.ReadOnly = true
		return meta, util.Errorf("%w", err)
	}

	// A wrong-typed icon reads as none, but the file stays writable: setting an
	// icon replaces exactly that member.
	var icon string
	if _, err := obj.Lookup(folderMetaIconKey, &icon); err == nil {
		meta.Icon = icon
	}
	// json/v2 accepts null into float64 as zero, but null means unspecified here.
	if raw, ok := obj[folderMetaSortKeyKey]; ok && strings.TrimSpace(string(raw)) != "null" {
		var key float64
		if _, err := obj.Lookup(folderMetaSortKeyKey, &key); err == nil && !math.IsNaN(key) && !math.IsInf(key, 0) {
			meta.SortKey, meta.HasSortKey = key, true
		}
	}
	return meta, nil
}

// SetFolderSortKey changes only sort_key, preserving icons and unknown members.
func (s *Shelf) SetFolderSortKey(folder FolderPath, key float64, version string) error {
	if math.IsNaN(key) || math.IsInf(key, 0) {
		return util.NewError("sort_key must be finite")
	}
	return s.updateFolderMeta(folder, version, func(obj jsonopt.Object) error { return obj.Set(folderMetaSortKeyKey, key) })
}

// SetFolderIcon sets folder's icon, or clears it when icon is "". version is the
// FolderMeta.Version the caller read; a different file on disk now fails with
// ErrFolderMetaConflict. Every other member of folder.json is kept.
func (s *Shelf) SetFolderIcon(folder FolderPath, icon string, version string) error {
	if icon != "" && !IsFolderIcon(icon) {
		return util.Errorf("%w: %q", ErrInvalidFolderIcon, icon)
	}
	return s.updateFolderMeta(folder, version, func(obj jsonopt.Object) error {
		if icon == "" {
			obj.Delete(folderMetaIconKey)
			return nil
		}
		return obj.Set(folderMetaIconKey, icon)
	})
}

// updateFolderMeta applies mutate to folder's folder.json under the shelf lock.
//
// The file is re-read under the lock and compared against version, so a change
// made since the caller's read — by another PlainShelf process sharing the
// lock, or by hand — is refused rather than overwritten. A write by a process
// that does not take the lock can still land between this read and the rename;
// nothing short of a lock it honours closes that.
//
// A file left with nothing but schema_version is removed rather than kept, so
// clearing the last setting leaves the folder as it was before it had any.
func (s *Shelf) updateFolderMeta(folder FolderPath, version string, mutate func(jsonopt.Object) error) error {
	if len(folder) == 0 {
		return util.Errorf("%w: the shelf root has no folder metadata", ErrInvalidFolder)
	}
	if err := s.ValidateFolderPath(folder); err != nil {
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

	dirPath := path.Join(booksFolder, path.Join(folder...))
	info, err := s.dbRoot.Stat(dirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return util.Errorf("%w: %w", ErrFolderNotFound, err)
		}
		return util.Errorf("%w", err)
	}
	if !info.IsDir() {
		return util.Errorf("%w: %s is not a folder", ErrInvalidFolder, folder)
	}

	metaPath := path.Join(dirPath, FolderMetaFile)
	data, err := readFolderMetaFile(s.dbRoot, metaPath)
	if err != nil {
		return util.Errorf("%w: %w", ErrFolderMetaNotWritable, err)
	}
	if folderMetaVersion(data) != version {
		// The cache missed this edit (made by hand, or by another process), so
		// the caller's next listing would hand back the same stale version.
		current, _ := folderMetaFromData(data)
		s.cacheFolderMeta(folder, current)
		return util.Errorf("%w", ErrFolderMetaConflict)
	}

	obj := jsonopt.Object{}
	if data != nil {
		if obj, err = parseFolderMeta(data); err != nil {
			return util.Errorf("%w: %w", ErrFolderMetaNotWritable, err)
		}
	}

	if err := mutate(obj); err != nil {
		return util.Errorf("%w", err)
	}

	if len(obj) == 0 || (len(obj) == 1 && obj[folderMetaSchemaVersionKey] != nil) {
		if data == nil {
			return nil
		}
		if err := root.Remove(metaPath); err != nil {
			return util.Errorf("%w", err)
		}
		s.cacheFolderMeta(folder, FolderMeta{})
		return nil
	}

	if err := obj.Set(folderMetaSchemaVersionKey, FolderMetaSchemaVersion); err != nil {
		return util.Errorf("%w", err)
	}
	payload, err := obj.MarshalDisk()
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := fsutil.WriteFileAtomic(root, metaPath, payload); err != nil {
		return util.Errorf("%w", err)
	}

	// Recorded rather than left to the next scan, like a new folder: the change
	// was just made here and has to show in the very next listing.
	written, _ := folderMetaFromData(payload)
	s.cacheFolderMeta(folder, written)
	return nil
}

// ListFolderMeta returns the cached metadata of every folder that has a
// folder.json, keyed like the folder list ("Fiction/Classics"). It is as fresh
// as GetAllFolders: a change made here shows at once, one made outside
// PlainShelf after the next full scan.
func (s *Shelf) ListFolderMeta() (map[string]FolderMeta, error) {
	if !s.IsReady() {
		return nil, s.notReady()
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	s.bookCache.RLock()
	defer s.bookCache.RUnlock()
	return maps.Clone(s.bookCache.folderMeta), nil
}

// cacheFolderMeta records folder's metadata in the book cache; the zero value
// removes the entry.
func (s *Shelf) cacheFolderMeta(folder FolderPath, meta FolderMeta) {
	s.bookCache.Lock()
	defer s.bookCache.Unlock()

	if meta == (FolderMeta{}) {
		delete(s.bookCache.folderMeta, folder.String())
		return
	}
	if s.bookCache.folderMeta == nil {
		s.bookCache.folderMeta = map[string]FolderMeta{}
	}
	s.bookCache.folderMeta[folder.String()] = meta
}

// readFolderMetaForScan reads the folder.json of every folder a walk found one
// in. Only those are read: the walk saw the file in its parent's listing, so a
// folder without one costs nothing, and a file's content is re-read on every
// full scan because an edit in place does not move its parent's mtime.
func (s *Shelf) readFolderMetaForScan(root fsutil.ReadFS, folders []FolderPath) (map[string]FolderMeta, error) {
	metas := make(map[string]FolderMeta, len(folders))
	for _, folder := range folders {
		meta, err := s.loadFolderMeta(root, folder)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		if meta != (FolderMeta{}) {
			metas[folder.String()] = meta
		}
	}
	return metas, nil
}

func folderMetaPath(folder FolderPath) string {
	return path.Join(booksFolder, path.Join(folder...), FolderMetaFile)
}

// readFolderMetaFile returns the file's bytes, nil when there is no file. A
// directory of that name, or a file over maxFolderMetaBytes, is an error.
func readFolderMetaFile(root fsutil.ReadFS, metaPath string) ([]byte, error) {
	file, err := root.Open(metaPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}
	defer file.Close() //nolint:errcheck // read-only

	if info, err := file.Stat(); err == nil && info.IsDir() {
		return nil, util.Errorf("%s is a directory", FolderMetaFile)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxFolderMetaBytes+1))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if len(data) > maxFolderMetaBytes {
		return nil, util.Errorf("%s is larger than %d bytes", FolderMetaFile, maxFolderMetaBytes)
	}
	return data, nil
}

// parseFolderMeta decodes a folder.json this build may write back: an object
// whose schema_version, when present, is FolderMetaSchemaVersion.
func parseFolderMeta(data []byte) (jsonopt.Object, error) {
	obj, err := jsonopt.ParseObject(data)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var version int
	found, err := obj.Lookup(folderMetaSchemaVersionKey, &version)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if found && version != FolderMetaSchemaVersion {
		return nil, util.Errorf("schema_version %d is not %d, the version this build reads", version, FolderMetaSchemaVersion)
	}
	return obj, nil
}

func folderMetaVersion(data []byte) string {
	if data == nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// warnFolderMeta logs a problem with one folder.json, once per distinct
// problem: folder metadata is read on every listing, and the same broken file
// would otherwise repeat its warning each time.
func (s *Shelf) warnFolderMeta(metaPath, msg string, err error) {
	text := err.Error()
	if previous, loaded := s.folderMetaWarnings.Swap(metaPath, text); loaded && previous == text {
		return
	}
	s.Warn(msg, "path", metaPath, "error", err)
}

// validateNewFolderName refuses a name a new or moved folder may not take, on
// top of ValidateFolderPath. It is checked only where a name is placed, so a
// folder of that name made outside PlainShelf stays listed and can still be
// renamed away or deleted.
func validateNewFolderName(name string) error {
	if strings.EqualFold(name, FolderMetaFile) {
		return util.Errorf("%w", ErrReservedFolderName)
	}
	return nil
}
