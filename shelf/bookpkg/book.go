package bookpkg

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

const SourcesFolder = "sources"
const BookMetaFile = "book.json"

const CurrentSourceHintFile = "CURRENT_SOURCE.txt"
const LegacyCurrentSourceHintFile = "CURRENT_VERSION_LOCATION.txt"

const CurrentSourceHintTemplate = `[PlainShelf hint]
This book currently reads from:
%s

This file is a hint written by PlainShelf, not the source of truth: nothing
reads it back, and current_source in book.json is what the app follows. You
can safely delete it; it is rewritten the next time the current source changes.
`

const BookMetaSchemaVersion = 1

var ErrSourceNotFound = util.NewError("source not found")
var ErrInvalidIdentifierKey = util.NewError("identifier key cannot be empty")

const (
	MinStar = 0
	MaxStar = 5
)

var ErrInvalidStar = util.NewError("star must be between 0 and 5")

var ErrInvalidLanguageTag = util.NewError("language must be a BCP 47 tag")

const (
	BookFormatText     = shelfutil.BookFormatText
	BookFormatMarkdown = shelfutil.BookFormatMarkdown
)

var ErrInvalidBookFormat = util.NewError(`format must be "txt" or "md"`)

var ErrUnsupportedBookSchemaVersion = util.NewError("book.json schema version is newer than this build supports")

type Book struct {
	logger     logutil.Logger
	root       fsutil.ReadFS
	folderPath string

	metaMu   sync.RWMutex
	meta     *BookMeta
	metaStat *FileStat
}

type BookMeta struct {
	// SchemaVersion is managed by shelf: any value a caller supplies is
	// overwritten on write. Declared first so it marshals as the first key,
	// keeping the file self-describing when opened in a text editor.
	SchemaVersion int `json:"schema_version"`

	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Format      string            `json:"format,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Identifiers map[string]string `json:"identifiers,omitempty"`
	Cover       string            `json:"cover"`
	Authors     []string          `json:"authors"`
	Language    string            `json:"language"`
	Comments    string            `json:"comments"`
	Star        int               `json:"star"`

	// NSFW is omitted when false, so a shelf that marks nothing writes exactly
	// the book.json it wrote before.
	//
	// It can only add: a shelf may also mark a whole folder subtree in
	// shelf.json, and false here does not take a book out of one. This field is
	// the book's own half of the answer; Shelf.IsBookNSFW is the answer.
	NSFW bool `json:"nsfw,omitzero"`

	CreatedAt   util.JSONTime `json:"created_at,omitzero"`
	UpdatedAt   util.JSONTime `json:"updated_at,omitzero"`
	PublishedAt util.JSONDate `json:"published_at,omitzero"`

	// CurrentSource is managed by shelf; SetCurrentSource is the way in.
	CurrentSource string `json:"current_source"`
}

func (b *Book) IsStale() bool {
	b.metaMu.RLock()
	fromSnapshot := b.metaStat == nil
	b.metaMu.RUnlock()
	if fromSnapshot {
		return false
	}

	metaPath := path.Join(b.folderPath, BookMetaFile)

	currentMetaStat, err := getFileStat(b.root, metaPath)
	if err != nil {
		b.logger.Warn("failed to stat meta file during IsStale check, treating book as stale", "error", err)
		return true
	}

	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	return !currentMetaStat.Equal(b.metaStat)
}

func (b *Book) ID() string {
	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	return b.meta.ID
}

func (b *Book) Title() string {
	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	return b.meta.Title
}

func (b *Book) PackagePath() string {
	return b.folderPath
}

func (b *Book) CoverETag() string {
	cover := b.coverName()
	if cover == "" {
		return ""
	}
	return shelfutil.FileETag(b.root, path.Join(b.folderPath, cover))
}

func (b *Book) OpenCover() ([]byte, string, error) {
	cover := b.coverName()
	if cover == "" {
		return nil, "", nil
	}

	coverPath := path.Join(b.folderPath, cover)
	coverFile, err := b.root.Open(coverPath)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	defer coverFile.Close()

	coverData, err := io.ReadAll(coverFile)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}

	ext := path.Ext(cover)
	return coverData, ext, nil
}

func (b *Book) coverName() string {
	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	return b.meta.Cover
}

func (b *Book) writeRoot() (fsutil.FS, error) {
	root, err := fsutil.Writable(b.root)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return root, nil
}

func (b *Book) EnsureWritable() error {
	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	if b.meta.SchemaVersion > BookMetaSchemaVersion {
		return util.Errorf("%w: book.json is schema_version %d, this build writes %d",
			ErrUnsupportedBookSchemaVersion, b.meta.SchemaVersion, BookMetaSchemaVersion)
	}
	return nil
}

func (b *Book) SetCover(imageData []byte, ext string) error {
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	coverFilename := "cover" + ext
	coverPath := path.Join(b.folderPath, coverFilename)

	previousCover := b.coverName()

	err = fsutil.WriteFileAtomic(root, coverPath, imageData)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.Cover = coverFilename
	err = b.SetMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if previousCover != "" && previousCover != coverFilename {
		b.removeCoverIfUnreferenced(root, previousCover)
	}

	return nil
}

func (b *Book) removeCoverIfUnreferenced(root fsutil.FS, previousCover string) {
	persisted, err := readBookMeta(b.root, b.folderPath)
	if err != nil {
		b.logger.Warn("failed to re-read book meta before removing replaced cover", "cover", previousCover, "error", err)
		return
	}

	if persisted.Cover == previousCover {
		return
	}

	if err := root.Remove(path.Join(b.folderPath, previousCover)); err != nil {
		b.logger.Warn("failed to remove replaced cover file", "cover", previousCover, "error", err)
	}
}

func (b *Book) DeleteCover() error {
	cover := b.coverName()
	if cover == "" {
		return nil
	}

	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	coverPath := path.Join(b.folderPath, cover)
	err = root.Remove(coverPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.Cover = ""
	err = b.setMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) CurrentSource() string {
	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	return b.meta.CurrentSource
}

func (b *Book) SetCurrentSource(sourceID string) error {
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	source, err := b.GetSource(sourceID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.CurrentSource = sourceID
	mirrorSourceFormatIntoBookMeta(meta, source)

	err = b.setMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = b.writeCurrentSourceHint(sourceID)
	if err != nil {
		b.logger.Warn("failed to write current source hint", "error", err)
	}

	return nil
}

func mirrorSourceFormatIntoBookMeta(meta *BookMeta, source *Source) {
	if sourceFormat := source.GetMeta().Format; sourceFormat != "" {
		meta.Format = sourceFormat
	}
}

func (b *Book) writeCurrentSourceHint(sourceID string) error {
	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	sourcePath := path.Join(SourcesFolder, sourceID, SourceFile)
	hintContent := fmt.Sprintf(CurrentSourceHintTemplate, sourcePath)

	hintPath := path.Join(b.folderPath, CurrentSourceHintFile)

	err = fsutil.WriteFileAtomic(root, hintPath, []byte(hintContent))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = root.Remove(path.Join(b.folderPath, LegacyCurrentSourceHintFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) GetMeta() *BookMeta {
	b.metaMu.RLock()
	defer b.metaMu.RUnlock()
	return cloneBookMeta(b.meta)
}

func cloneBookMeta(meta *BookMeta) *BookMeta {
	metaCopy := *meta
	metaCopy.Tags = slices.Clone(meta.Tags)
	metaCopy.Authors = slices.Clone(meta.Authors)
	metaCopy.Identifiers = maps.Clone(meta.Identifiers)
	return &metaCopy
}

func (b *Book) SetMeta(meta *BookMeta) error {
	if meta.CurrentSource != b.CurrentSource() {
		return util.NewError("cannot modify CurrentSource field directly, use SetCurrentSource method instead")
	}
	return b.setMeta(meta)
}

func (b *Book) setMeta(meta *BookMeta) error {
	if meta == nil {
		return util.NewError("meta cannot be nil")
	}

	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !shelfutil.ValidateBCP47(meta.Language) {
		return util.Errorf("%w: got %q", ErrInvalidLanguageTag, meta.Language)
	}

	if meta.Star < MinStar || meta.Star > MaxStar {
		return util.Errorf("%w: got %d", ErrInvalidStar, meta.Star)
	}

	if !shelfutil.ValidateBookFormat(meta.Format) {
		return util.Errorf("%w: got %q", ErrInvalidBookFormat, meta.Format)
	}

	for key := range meta.Identifiers {
		if strings.TrimSpace(key) == "" {
			return util.Errorf("%w", ErrInvalidIdentifierKey)
		}
	}
	meta.SchemaVersion = BookMetaSchemaVersion

	bs, err := json.Marshal(meta, jsonopt.Disk())
	if err != nil {
		return util.Errorf("%w", err)
	}

	metaPath := path.Join(b.folderPath, BookMetaFile)

	err = fsutil.WriteFileAtomic(root, metaPath, bs)
	if err != nil {
		return util.Errorf("%w", err)
	}

	var metaStat *FileStat

	metaStat, err = getFileStat(b.root, metaPath)
	if err != nil {
		metaStat = &FileStat{
			ModTime: time.Now(),
			Size:    int64(len(bs)),
		}
		b.logger.Warn("failed to stat meta file after SetMeta, metaStat may be inaccurate", "error", err)
	}

	b.metaMu.Lock()
	b.meta = cloneBookMeta(meta)
	if b.metaStat != nil {
		b.metaStat = metaStat
	}
	b.metaMu.Unlock()

	return nil
}

type NewSourceOptions struct {
	Format  string
	Comment string
}

func (b *Book) NewSource(source io.Reader) (*Source, error) {
	return b.NewSourceWithOptions(source, NewSourceOptions{Format: BookFormatText})
}

func (b *Book) NewSourceWithOptions(source io.Reader, options NewSourceOptions) (*Source, error) {
	if err := b.EnsureWritable(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	root, err := b.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if source == nil {
		source = io.NopCloser(strings.NewReader(""))
	}
	if options.Format == "" {
		options.Format = BookFormatText
	}
	if !shelfutil.ValidateBookFormat(options.Format) {
		return nil, util.Errorf("%w: got %q", ErrInvalidBookFormat, options.Format)
	}

	sourceID, err := b.freeSourceID(time.Now().Format("20060102-150405"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)

	tempSourcePath := path.Join(b.folderPath, SourcesFolder, fsutil.TempDirName(sourceID))
	defer root.RemoveAll(tempSourcePath)

	src, err := createSource(root, tempSourcePath, sourceID, source, options.Format, options.Comment)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := root.Rename(tempSourcePath, sourcePath); err != nil {
		return nil, util.Errorf("%w", err)
	}
	src.folderPath = sourcePath

	return src, nil
}

func (b *Book) freeSourceID(base string) (string, error) {
	sourceID := base
	for i := 1; ; i++ {
		sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)
		if _, err := b.root.Stat(sourcePath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return sourceID, nil
			}
			return "", util.Errorf("%w", err)
		}
		sourceID = fmt.Sprintf("%s-%d", base, i)
	}
}

func (b *Book) GetSource(sourceID string) (*Source, error) {
	if err := shelfutil.ValidateSourceID(sourceID); err != nil {
		return nil, util.Errorf("%w", err)
	}

	sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)
	if _, err := b.root.Stat(sourcePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, util.Errorf("%w", ErrSourceNotFound)
		}
		return nil, util.Errorf("%w", err)
	}

	source, err := openSource(b.root, sourcePath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return source, nil
}

func (b *Book) CurrentSourceStat() (charCount int, md5 string) {
	sourceID := b.CurrentSource()
	if err := shelfutil.ValidateSourceID(sourceID); err != nil {
		return 0, ""
	}

	source, err := openSource(b.root, path.Join(b.folderPath, SourcesFolder, sourceID))
	if err != nil {
		return 0, ""
	}

	meta := source.GetMeta()
	return meta.CharCount, meta.MD5Hash
}

func (b *Book) ResolveCurrentSource() (*Source, error) {
	if currentID := b.CurrentSource(); currentID != "" {
		source, err := b.GetSource(currentID)
		if err == nil {
			return source, nil
		}
		b.logger.Warn("current source is unusable, falling back to the newest source",
			"book", b.folderPath, "current_source", currentID, "error", err)
	}

	sources, err := b.ListSource()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, util.Errorf("%w", ErrSourceNotFound)
		}
		return nil, util.Errorf("%w", err)
	}
	if len(sources) == 0 {
		return nil, util.Errorf("%w", ErrSourceNotFound)
	}

	return sources[len(sources)-1], nil
}

func (b *Book) latestSourceExcluding(excludeID string) (*Source, error) {
	sources, err := b.ListSource()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}

	var latest *Source
	for _, source := range sources {
		if source.ID() == excludeID {
			continue
		}
		if latest == nil || source.ID() > latest.ID() {
			latest = source
		}
	}

	return latest, nil
}

func (b *Book) DeleteSource(sourceID string) error {
	if err := shelfutil.ValidateSourceID(sourceID); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	deletingCurrent := sourceID == b.CurrentSource()
	if deletingCurrent {
		if err := b.EnsureWritable(); err != nil {
			return util.Errorf("%w", err)
		}
	}

	sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)
	if _, err := b.root.Stat(sourcePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return util.Errorf("%w", ErrSourceNotFound)
		}
		return util.Errorf("%w", err)
	}
	source, err := openSource(b.root, sourcePath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := source.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	if deletingCurrent {
		if err := b.handOverCurrentSource(sourceID); err != nil {
			return util.Errorf("%w", err)
		}
	}

	err = root.RemoveAll(sourcePath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) handOverCurrentSource(leavingID string) error {
	successor, err := b.latestSourceExcluding(leavingID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	successorID := ""
	if successor != nil {
		successorID = successor.ID()
	} else {
		replacement, err := b.NewSource(nil)
		if err != nil {
			return util.Errorf("%w", err)
		}
		successorID = replacement.ID()
	}

	if err := b.SetCurrentSource(successorID); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) ListSource() ([]*Source, error) {
	sourcesPath := path.Join(b.folderPath, SourcesFolder)

	sourceEntries, err := b.root.ReadDir(sourcesPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var sources []*Source
	for _, entry := range sourceEntries {
		if !entry.IsDir() {
			continue
		}

		revID := entry.Name()
		if strings.HasPrefix(revID, ".") && strings.HasSuffix(revID, ".tmp") {
			continue
		}
		sourcePath := path.Join(sourcesPath, revID)
		source, err := openSource(b.root, sourcePath)
		if err != nil {
			b.logger.Warn("skipping source that failed to open", "path", sourcePath, "error", err)
			continue
		}

		sources = append(sources, source)
	}

	slices.SortFunc(sources, func(a, c *Source) int {
		return strings.Compare(a.ID(), c.ID())
	})

	return sources, nil
}

func Open(rt fsutil.ReadFS, logger logutil.Logger, bookPath string) (*Book, error) {
	bookFolder, err := rt.Stat(bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if !bookFolder.IsDir() {
		return nil, util.Errorf("%s is not a book directory", bookPath)
	}

	metaPath := path.Join(bookPath, BookMetaFile)

	meta, err := readBookMeta(rt, bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	normalizeMetaVersion(logger, metaPath, meta)

	metaStat, err := getFileStat(rt, metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Book{
		root:       rt,
		folderPath: bookPath,
		meta:       meta,
		logger:     logger,

		metaStat: metaStat,
	}, nil
}

func FromSnapshot(rt fsutil.ReadFS, logger logutil.Logger, bookPath string, meta *BookMeta) *Book {
	normalizeMetaVersion(logger, path.Join(bookPath, BookMetaFile), meta)

	return &Book{
		root:       rt,
		folderPath: bookPath,
		meta:       meta,
		logger:     logger,
	}
}

func normalizeMetaVersion(logger logutil.Logger, metaPath string, meta *BookMeta) {
	switch {
	case meta.SchemaVersion > BookMetaSchemaVersion:
		logger.Warn("book.json schema version is newer than this build supports; book is read-only",
			"path", metaPath,
			"schema_version", meta.SchemaVersion,
			"supported", BookMetaSchemaVersion)
	case meta.SchemaVersion < BookMetaSchemaVersion:
		meta.SchemaVersion = BookMetaSchemaVersion
	}
}

func readBookMeta(rt fsutil.ReadFS, bookPath string) (*BookMeta, error) {
	metaPath := path.Join(bookPath, BookMetaFile)
	metaFile, err := rt.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta BookMeta
	if err := json.UnmarshalRead(metaFile, &meta); err != nil {
		return nil, util.Errorf("%w", NameMetadataDecodeError(metaPath, err))
	}

	return &meta, nil
}

func Create(rt fsutil.FS, logger logutil.Logger, bookPath, bookID, title string) (*Book, error) {
	err := rt.MkdirAll(bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	meta := BookMeta{
		SchemaVersion: BookMetaSchemaVersion,
		ID:            bookID,
		Title:         title,
		CreatedAt:     util.JSONTime(time.Now()),
	}

	bs, err := json.Marshal(meta, jsonopt.Disk())
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	metaFilePath := path.Join(bookPath, BookMetaFile)
	err = fsutil.WriteFileAtomic(rt, metaFilePath, bs)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var metaStat *FileStat
	metaStat, err = getFileStat(rt, metaFilePath)
	if err != nil {
		logger.Warn("failed to stat meta file after creating book, metaStat may be inaccurate", "error", err)
		metaStat = &FileStat{
			ModTime: time.Now(),
			Size:    int64(len(bs)),
		}
	}

	return &Book{
		root:       rt,
		folderPath: bookPath,
		meta:       &meta,
		logger:     logger,

		metaStat: metaStat,
	}, nil
}
