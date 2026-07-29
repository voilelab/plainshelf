package shelf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

/*
{book-folder}/
├─ book.json
├─ CURRENT_VERSION_LOCATION.txt
├─ cover.(jpg|png|webp)
└─ sources/
   └─ {source-id}
*/

const SourcesFolder = "sources"
const BookMetaFile = "book.json"
const CurrentVersionLocationFile = "CURRENT_VERSION_LOCATION.txt"
const CurrentVersionLocationTemplate = `[shelf 狀態指標]
當前閱讀版本存放於：
%s

(註：請勿修改此檔案內容，shelf 會自動更新此指標)
`

// BookMetaSchemaVersion is the book.json schema version this build writes.
//
// A book.json with no schema_version field predates versioning ("v0"): it is
// read as v1 and normalized in memory, and the version is only persisted the
// next time the book is written (lazy upgrade, same pattern as published_at in
// internal/util/json_date.go). Opening a library never rewrites it.
//
// A book.json with a HIGHER schema_version is read best-effort but is never
// written back, so an older build cannot clobber data written by a newer one.
// That refusal is enforced by EnsureWritable, which every mutating operation
// calls before touching the filesystem.
const BookMetaSchemaVersion = 1

var ErrSourceNotFound = util.NewError("source not found")
var ErrInvalidIdentifierKey = util.NewError("identifier key cannot be empty")

// ErrUnsupportedBookSchemaVersion is returned when a write is attempted against
// a book.json whose on-disk schema_version is newer than this build supports.
// It is book.json specific on purpose: sources/{id}/meta.json and trash.json
// will need their own sentinels so errors.Is can tell which file is too new.
var ErrUnsupportedBookSchemaVersion = util.NewError("book.json schema version is newer than this build supports")

type Layers []string

func (l Layers) String() string {
	return strings.Join(l, "/")
}

func (l Layers) Equal(other Layers) bool {
	if len(l) != len(other) {
		return false
	}
	for i := range l {
		if l[i] != other[i] {
			return false
		}
	}
	return true
}

func NewLayersFromString(s string) Layers {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

type Book struct {
	logger     logutil.Logger
	root       fsutil.FS
	folderPath string
	meta       *BookMeta
	layers     Layers

	metaStat FileStat
}

type BookMeta struct {
	// SchemaVersion is the on-disk format version of book.json. It is managed by
	// shelf: any value supplied by a caller is ignored and overwritten with
	// BookMetaSchemaVersion on write. Declared first so it marshals as the first
	// key, keeping the file self-describing when opened in a text editor.
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
	CreatedAt   util.JSONTime     `json:"created_at,omitzero"`
	UpdatedAt   util.JSONTime     `json:"updated_at,omitzero"`
	PublishedAt util.JSONDate     `json:"published_at,omitzero"`

	// User should not modify CurrentSource directly, it is managed by shelf internally,
	// and can be updated via SetCurrentSource method
	CurrentSource string `json:"current_source"`
}

// setLayers only used for internal use, not persisted in book meta, and not exposed to user
func (b *Book) setLayers(layers Layers) {
	b.layers = layers
}

// IsStale checks whether the cached book metadata is out of date by comparing
// the current file stat of the book meta file with the cached metaStat. If the
// file stat differs, the book is considered stale and should be refreshed.
func (b *Book) IsStale() bool {
	// FIXME: IsStale only treats the cache as stale when the tracked book.json
	// file stat changes. If the file content changes but preserves the same
	// tracked stat values (for example, mtime and size), the change won’t be detected.
	metaPath := path.Join(b.folderPath, BookMetaFile)

	currentMetaStat, err := getFileStat(b.root, metaPath)
	if err != nil {
		// If we cannot stat the meta file, we consider the book as stale to be safe,
		// and let the caller handle the error when trying to open the book.
		b.logger.Warn("failed to stat meta file during IsStale check, treating book as stale", "error", err)
		return true
	}

	if !currentMetaStat.Equal(&b.metaStat) {
		return true
	}

	return false
}

func (b *Book) Layers() Layers {
	return b.layers
}

func (b *Book) ID() string {
	return b.meta.ID
}

func (b *Book) Title() string {
	return b.meta.Title
}

func (b *Book) FolderPath() string {
	return b.folderPath
}

// CoverETag returns a weak ETag derived from the cover file's mtime and size.
// Returns an empty string if the book has no cover or the stat fails.
func (b *Book) CoverETag() string {
	if b.meta.Cover == "" {
		return ""
	}
	info, err := b.root.Stat(path.Join(b.folderPath, b.meta.Cover))
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`W/"%d-%d"`, info.ModTime().UnixNano(), info.Size())
}

func (b *Book) OpenCover() ([]byte, string, error) {
	if b.meta.Cover == "" {
		return nil, "", nil
	}

	coverPath := path.Join(b.folderPath, b.meta.Cover)
	coverFile, err := b.root.Open(coverPath)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	defer coverFile.Close()

	coverData, err := io.ReadAll(coverFile)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}

	ext := path.Ext(b.meta.Cover)
	return coverData, ext, nil
}

// EnsureWritable reports an error when the book's on-disk schema version is
// newer than this build supports, meaning the book must be treated as
// read-only.
//
// Call it before any filesystem mutation on the book, not only before writing
// book.json: a guard that runs last still lets a refused request truncate a
// cover, delete a file, or rename a folder first. It is checked against b.meta
// — what is actually on disk — so a caller-supplied BookMeta cannot bypass it.
func (b *Book) EnsureWritable() error {
	if b.meta.SchemaVersion > BookMetaSchemaVersion {
		return util.Errorf("%w: book.json is schema_version %d, this build writes %d",
			ErrUnsupportedBookSchemaVersion, b.meta.SchemaVersion, BookMetaSchemaVersion)
	}
	return nil
}

func (b *Book) SetCover(imageData []byte, ext string) error {
	// Guard before OpenWriter: it creates or truncates the cover file, so
	// refusing only at SetMeta would already have destroyed the existing cover.
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	coverFilename := "cover" + ext
	coverPath := path.Join(b.folderPath, coverFilename)

	coverFile, err := b.root.OpenWriter(coverPath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer coverFile.Close()

	_, err = coverFile.Write(imageData)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.Cover = coverFilename
	err = b.SetMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (b *Book) DeleteCover() error {
	if b.meta.Cover == "" {
		return nil
	}

	// Guard before Remove: refusing only at setMeta would leave the cover
	// deleted while book.json still references it.
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	coverPath := path.Join(b.folderPath, b.meta.Cover)
	err := b.root.Remove(coverPath)
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
	return b.meta.CurrentSource
}

func (b *Book) SetCurrentSource(sourceID string) error {
	meta := b.GetMeta()
	meta.CurrentSource = sourceID

	err := b.setMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = b.updateCurrentVersionLocation(sourceID)
	if err != nil {
		// This error is not critical, just log it and continue.
		b.logger.Warn("failed to update current version location", "error", err)
	}

	return nil
}

func (b *Book) updateCurrentVersionLocation(sourceID string) error {
	sourcePath := path.Join(SourcesFolder, sourceID, SourceFile)
	sourceContent := fmt.Sprintf(CurrentVersionLocationTemplate, sourcePath)

	currentVersionLocationPath := path.Join(b.folderPath, CurrentVersionLocationFile)
	tmpCurrentVersionLocationPath := currentVersionLocationPath + ".tmp"

	err := b.root.WriteFile(tmpCurrentVersionLocationPath, []byte(sourceContent))
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = b.root.Rename(tmpCurrentVersionLocationPath, currentVersionLocationPath)
	if err != nil {
		err2 := b.root.Remove(tmpCurrentVersionLocationPath)
		if err2 != nil {
			b.logger.Warn("failed to remove temp current version location file", "error", err2)
		}
		return util.Errorf("%w", err)
	}

	return nil
}

// GetMeta returns a copy of the book meta, user can modify the returned meta and call SetMeta to update the book meta, but should not modify the CurrentSource field directly
func (b *Book) GetMeta() *BookMeta {
	metaCopy := *b.meta
	metaCopy.Tags = append([]string(nil), b.meta.Tags...)
	metaCopy.Authors = append([]string(nil), b.meta.Authors...)
	metaCopy.Identifiers = maps.Clone(b.meta.Identifiers)
	return &metaCopy
}

// SetMeta allows user to update book meta, but not the CurrentSource field which is managed by shelf internally
func (b *Book) SetMeta(meta *BookMeta) error {
	if meta.CurrentSource != b.meta.CurrentSource {
		return util.NewError("cannot modify CurrentSource field directly, use SetCurrentSource method instead")
	}

	return b.setMeta(meta)
}

func (b *Book) setMeta(meta *BookMeta) error {
	if meta == nil {
		return util.NewError("meta cannot be nil")
	}

	// Never let this build overwrite a book written by a newer one. Callers that
	// touch other files first guard earlier; this is the backstop for book.json.
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	if !validateBCP47(meta.Language) {
		return util.Errorf("invalid language tag: %s", meta.Language)
	}

	if meta.Star < 0 || meta.Star > 5 {
		return util.Errorf("invalid star rating: %d", meta.Star)
	}

	for key := range meta.Identifiers {
		if strings.TrimSpace(key) == "" {
			return util.Errorf("%w", ErrInvalidIdentifierKey)
		}
	}

	// Stamp unconditionally: the schema version is shelf-managed and any value
	// the caller supplied is ignored. This is what makes the v0 → v1 upgrade
	// lazy — the version reaches disk only when the book is next written.
	meta.SchemaVersion = BookMetaSchemaVersion

	// write back to book meta
	bs, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return util.Errorf("%w", err)
	}

	metaPath := path.Join(b.folderPath, BookMetaFile)

	// write to a temp file first, then rename to ensure atomic update
	// When syncing to remote storage, the software should not sync the temp file,
	// and only sync the final meta file after rename is successful,
	// to avoid syncing incomplete meta file
	tmpMetaPath := metaPath + ".tmp"

	err = b.root.WriteFile(tmpMetaPath, bs)
	if err != nil {
		return util.Errorf("%w", err)
	}

	err = b.root.Rename(tmpMetaPath, metaPath)
	if err != nil {
		err2 := b.root.Remove(tmpMetaPath)
		if err2 != nil {
			b.logger.Warn("failed to remove temp meta file", "error", err2)
		}
		return util.Errorf("%w", err)
	}

	b.meta = meta

	var metaStat *FileStat

	metaStat, err = getFileStat(b.root, metaPath)
	if err != nil {
		metaStat = &FileStat{
			ModTime: time.Now(),
			Size:    int64(len(bs)),
		}
		b.logger.Warn("failed to stat meta file after SetMeta, metaStat may be inaccurate", "error", err)
	}

	b.metaStat = *metaStat

	return nil
}

// NewSource creates a new source for the book with the provided source content, and returns the created source metadata.
// If source is nil, an empty source will be created.
func (b *Book) NewSource(source io.Reader) (*Source, error) {
	if source == nil {
		source = io.NopCloser(strings.NewReader(""))
	}

	// create a new source for the given book with the provided source file and metadata.
	// The base ID is a second-granularity timestamp, so two sources created within the
	// same second would otherwise collide and overwrite each other. Probe for a free
	// folder name and bump a numeric suffix on collision (same scheme as NewBook).
	baseSourceID := time.Now().Format("20060102-150405")
	sourceID := baseSourceID
	var sourcePath string
	for i := 1; ; i++ {
		sourcePath = path.Join(b.folderPath, SourcesFolder, sourceID)
		if _, err := b.root.Stat(sourcePath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				break
			}
			return nil, util.Errorf("%w", err)
		}
		sourceID = fmt.Sprintf("%s-%d", baseSourceID, i)
	}

	src, err := createSource(b.root, b.logger, sourcePath, sourceID, source)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	err = b.updateCurrentVersionLocation(sourceID)
	if err != nil {
		// This error is not critical, just log it and continue.
		b.logger.Warn("failed to update current version location", "error", err)
	}

	return src, nil
}

func (b *Book) GetSource(sourceID string) (*Source, error) {
	if err := validateSourceID(sourceID); err != nil {
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

func (b *Book) DeleteSource(sourceID string) error {
	if err := validateSourceID(sourceID); err != nil {
		return util.Errorf("%w", err)
	}

	sourcePath := path.Join(b.folderPath, SourcesFolder, sourceID)
	if _, err := b.root.Stat(sourcePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return util.Errorf("%w", ErrSourceNotFound)
		}
		return util.Errorf("%w", err)
	}

	err := b.root.RemoveAll(sourcePath)
	if err != nil {
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
		// Skip non-directory entries (e.g. leftover *.tmp files from an
		// interrupted atomic write, or stray files created by a sync tool) so
		// a single bad entry does not fail the whole listing.
		if !entry.IsDir() {
			continue
		}

		revID := entry.Name()
		sourcePath := path.Join(sourcesPath, revID)
		source, err := openSource(b.root, sourcePath)
		if err != nil {
			b.logger.Warn("skipping source that failed to open", "path", sourcePath, "error", err)
			continue
		}

		sources = append(sources, source)
	}

	return sources, nil
}

func openBook(rt fsutil.FS, logger logutil.Logger, bookPath string) (*Book, error) {
	bookFolder, err := rt.Stat(bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if !bookFolder.IsDir() {
		return nil, util.Errorf("%s is not a book directory", bookPath)
	}

	metaPath := path.Join(bookPath, BookMetaFile)
	metaFile, err := rt.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta BookMeta
	decoder := json.NewDecoder(metaFile)
	if err := decoder.Decode(&meta); err != nil {
		return nil, util.Errorf("%w", err)
	}

	// A too-new book is deliberately NOT an error here. Failing would make the
	// book vanish from listings (iterateBooks), get evicted from the cache
	// (onlyRefreshBooksInCache), 404 from the API, and — worst — become
	// impossible to restore from trash. A visible, explained book beats a book
	// that silently disappears. setMeta is what protects the bytes on disk.
	switch {
	case meta.SchemaVersion > BookMetaSchemaVersion:
		logger.Warn("book.json schema version is newer than this build supports; book is read-only",
			"path", metaPath,
			"schema_version", meta.SchemaVersion,
			"supported", BookMetaSchemaVersion)
	case meta.SchemaVersion < BookMetaSchemaVersion:
		// Missing (pre-v1), zero, or garbage. Normalize in memory only; the
		// version reaches disk on the next write. Future versions add real
		// field migration here.
		meta.SchemaVersion = BookMetaSchemaVersion
	}

	metaStat, err := getFileStat(rt, metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &Book{
		root:       rt,
		folderPath: bookPath,
		meta:       &meta,
		logger:     logger,

		metaStat: *metaStat,
	}, nil
}

func createBook(rt fsutil.FS, logger logutil.Logger, bookPath, bookID, title string) (*Book, error) {
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

	bs, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	metaFilePath := path.Join(bookPath, BookMetaFile)
	tmpMetaFilePath := metaFilePath + ".tmp"
	err = rt.WriteFile(tmpMetaFilePath, bs)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	err = rt.Rename(tmpMetaFilePath, metaFilePath)
	if err != nil {
		err2 := rt.Remove(tmpMetaFilePath)
		if err2 != nil {
			logger.Warn("failed to remove temp meta file after failed rename, meta file may be left in an inconsistent state", "error", err2)
		}
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

		metaStat: *metaStat,
	}, nil
}
