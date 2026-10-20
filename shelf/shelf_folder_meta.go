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

const FolderMetaFile = "folder.json"

const FolderMetaSchemaVersion = 1

const maxFolderMetaBytes = 64 << 10

const (
	folderMetaSchemaVersionKey = "schema_version"
	folderMetaIconKey          = "icon"
	folderMetaSortKeyKey       = "sort_key"
)

var ErrReservedFolderName = util.Errorf("%w: %q is reserved for folder metadata", ErrInvalidFolder, FolderMetaFile)

var ErrInvalidFolderIcon = util.NewError("folder icon is not one this build offers")

var ErrFolderMetaNotWritable = util.NewError("folder.json cannot be updated by this build")

var ErrFolderNotFound = util.NewError("folder not found")

var ErrFolderMetaConflict = util.NewError("folder.json changed since it was read")

var folderIcons = []string{
	"alien", "atom", "bolt", "book", "bookmark", "books", "briefcase", "camera",
	"code", "coffee", "compass", "crown", "device-gamepad-2", "diamond", "flame",
	"flask", "flower", "folder", "ghost", "heart", "leaf", "map", "mood-smile",
	"moon", "movie", "music", "notebook", "palette", "paw", "planet", "puzzle",
	"robot", "rocket", "school", "shield", "skull", "star", "sun", "sword", "world",
}

func FolderIcons() []string {
	return slices.Clone(folderIcons)
}

func IsFolderIcon(name string) bool {
	_, found := slices.BinarySearch(folderIcons, name)
	return found
}

type FolderMeta struct {
	SortKey    float64
	HasSortKey bool

	Icon string

	ReadOnly bool

	Version string
}

func (m FolderMeta) DisplayIcon() string {
	if IsFolderIcon(m.Icon) {
		return m.Icon
	}
	return ""
}

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

func (s *Shelf) SetFolderSortKey(folder FolderPath, key float64, version string) error {
	if math.IsNaN(key) || math.IsInf(key, 0) {
		return util.NewError("sort_key must be finite")
	}
	return s.updateFolderMeta(folder, version, func(obj jsonopt.Object) error { return obj.Set(folderMetaSortKeyKey, key) })
}

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

	written, _ := folderMetaFromData(payload)
	s.cacheFolderMeta(folder, written)
	return nil
}

func (target *Shelf) CopyFolderMetaFrom(source *Shelf, from, to FolderPath) (string, error) {
	if len(from) == 0 || len(to) == 0 {
		return "", util.Errorf("%w: the shelf root has no folder metadata", ErrInvalidFolder)
	}
	if source == target {
		return "", util.Errorf("%w", ErrSameShelfTransfer)
	}
	if err := source.ValidateFolderPath(from); err != nil {
		return "", util.Errorf("%w", err)
	}
	if err := target.ValidateFolderPath(to); err != nil {
		return "", util.Errorf("%w", err)
	}

	data, err := source.readFolderMetaLocked(from)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", nil
	}

	root, err := target.writeRoot()
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	if err := target.shelfLock.Lock(); err != nil {
		return "", util.Errorf("%w", err)
	}
	defer target.shelfLock.Unlock()

	dirPath := path.Join(booksFolder, path.Join(to...))
	info, err := target.dbRoot.Stat(dirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", util.Errorf("%w: %w", ErrFolderNotFound, err)
		}
		return "", util.Errorf("%w", err)
	}
	if !info.IsDir() {
		return "", util.Errorf("%w: %s is not a folder", ErrInvalidFolder, to)
	}

	metaPath := path.Join(dirPath, FolderMetaFile)
	if _, err := target.dbRoot.Stat(metaPath); err == nil {
		return "", util.Errorf("%w: %s already has one", ErrFolderMetaConflict, to)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", util.Errorf("%w", err)
	}
	if err := fsutil.WriteFileAtomic(root, metaPath, data); err != nil {
		return "", util.Errorf("%w", err)
	}

	meta, _ := folderMetaFromData(data)
	target.cacheFolderMeta(to, meta)
	return meta.Version, nil
}

func (s *Shelf) readFolderMetaLocked(folder FolderPath) ([]byte, error) {
	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	data, err := readFolderMetaFile(s.dbRoot, folderMetaPath(folder))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return data, nil
}

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

func readFolderMetaFile(root fsutil.ReadFS, metaPath string) ([]byte, error) {
	file, err := root.Open(metaPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}
	defer file.Close()

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

func (s *Shelf) warnFolderMeta(metaPath, msg string, err error) {
	text := err.Error()
	if previous, loaded := s.folderMetaWarnings.Swap(metaPath, text); loaded && previous == text {
		return
	}
	s.Warn(msg, "path", metaPath, "error", err)
}

func validateNewFolderName(name string) error {
	if strings.EqualFold(name, FolderMetaFile) {
		return util.Errorf("%w", ErrReservedFolderName)
	}
	return nil
}
