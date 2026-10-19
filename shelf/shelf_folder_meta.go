package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
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
)

// ErrReservedFolderName refuses a folder named after FolderMetaFile, which would
// collide with its parent's metadata.
var ErrReservedFolderName = util.Errorf("%w: %q is reserved for folder metadata", ErrInvalidFolder, FolderMetaFile)

// ErrInvalidFolderIcon refuses an icon name that is not in FolderIcons.
var ErrInvalidFolderIcon = util.NewError("folder icon is not one this build offers")

// ErrFolderMetaNotWritable refuses a write to a folder.json this build cannot
// read safely: malformed, not an object, or from a newer schema version.
var ErrFolderMetaNotWritable = util.NewError("folder.json cannot be updated by this build")

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

// ReadFolderMeta reads folder's folder.json. Every problem with the file reads
// as no metadata and is logged once per distinct problem, never as an error: a
// listing must not fail on a hand-edited file. The error is for an invalid path
// or a shelf that cannot be read at all.
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

	metaPath := folderMetaPath(folder)
	data, err := readFolderMetaFile(s.dbRoot, metaPath)
	if err != nil {
		if errors.Is(err, fsutil.ErrSourceUnavailable) {
			return FolderMeta{}, util.Errorf("%w", err)
		}
		s.warnFolderMeta(metaPath, "failed to read folder metadata, showing the folder without it", err)
		return FolderMeta{ReadOnly: true}, nil
	}
	if data == nil {
		s.folderMetaWarnings.Delete(metaPath)
		return FolderMeta{}, nil
	}

	meta := FolderMeta{Version: folderMetaVersion(data)}
	obj, err := parseFolderMeta(data)
	if err != nil {
		s.warnFolderMeta(metaPath, "folder metadata cannot be read, showing the folder without it", err)
		meta.ReadOnly = true
		return meta, nil
	}
	s.folderMetaWarnings.Delete(metaPath)

	// A wrong-typed icon reads as none, but the file stays writable: setting an
	// icon replaces exactly that member.
	var icon string
	if _, err := obj.Lookup(folderMetaIconKey, &icon); err == nil {
		meta.Icon = icon
	}
	return meta, nil
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
	return nil
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
