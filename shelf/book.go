package shelf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
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

// MinStar and MaxStar bound BookMeta.Star, inclusive.
const (
	MinStar = 0
	MaxStar = 5
)

var ErrInvalidStar = util.NewError("star must be between 0 and 5")

// ErrInvalidLanguageTag is returned when BookMeta.Language is neither empty nor
// a well-formed BCP 47 tag.
var ErrInvalidLanguageTag = util.NewError("language must be a BCP 47 tag")

// BookFormatText and BookFormatMarkdown are the values BookMeta.Format accepts.
// They decide how the reader renders the book's text; the bytes on disk are the
// same either way, so switching between them rewrites nothing but book.json.
const (
	BookFormatText     = "txt"
	BookFormatMarkdown = "md"
)

// ErrInvalidBookFormat is returned when BookMeta.Format is neither empty nor a
// known format.
var ErrInvalidBookFormat = util.NewError(`format must be "txt" or "md"`)

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
	return fileETag(b.root, path.Join(b.folderPath, b.meta.Cover))
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
	// Guard before touching the filesystem: refusing only at SetMeta would
	// already have published the new cover file. The atomic write below keeps
	// the previous image intact either way, but a read-only book should not
	// gain a new cover file at all.
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	coverFilename := "cover" + ext
	coverPath := path.Join(b.folderPath, coverFilename)

	previousCover := b.meta.Cover

	// Write the image before pointing the meta at it, so the meta never
	// references a half-written cover file.
	err := fsutil.WriteFileAtomic(b.root, coverPath, imageData)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.Cover = coverFilename
	err = b.SetMeta(meta)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// A different extension leaves the old image behind unreferenced (the API
	// converts uploads to JPEG, so cover.png -> cover.jpg is a normal path).
	// The shelf is meant to be browsable by hand, so don't leave the orphan.
	if previousCover != "" && previousCover != coverFilename {
		b.removeReplacedCover(previousCover)
	}

	return nil
}

// removeReplacedCover deletes a cover file this call replaced, but only if the
// book still points away from it.
//
// An overlapping upload can write a new image under the old name and point the
// book back at it - starting from cover.png, a JPEG upload and a concurrent PNG
// upload can interleave that way. Deleting it then would leave book.json
// referencing a file that no longer exists, so the persisted meta is re-read
// and the file is left alone if it has been claimed again. Losing an orphan is
// cheaper than losing the cover the book actually points to.
func (b *Book) removeReplacedCover(previousCover string) {
	persisted, err := readBookMeta(b.root, b.folderPath)
	if err != nil {
		b.logger.Warn("failed to re-read book meta before removing replaced cover", "cover", previousCover, "error", err)
		return
	}

	if persisted.Cover == previousCover {
		return
	}

	if err := b.root.Remove(path.Join(b.folderPath, previousCover)); err != nil {
		b.logger.Warn("failed to remove replaced cover file", "cover", previousCover, "error", err)
	}
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
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	source, err := b.GetSource(sourceID)
	if err != nil {
		return util.Errorf("%w", err)
	}

	meta := b.GetMeta()
	meta.CurrentSource = sourceID
	if sourceFormat := source.GetMeta().Format; sourceFormat != "" {
		// Compatibility mirror for clients that still read book.json directly.
		// New clients use the current source's meta.json as the authority.
		meta.Format = sourceFormat
	}

	err = b.setMeta(meta)
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

	err := fsutil.WriteFileAtomic(b.root, currentVersionLocationPath, []byte(sourceContent))
	if err != nil {
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
		return util.Errorf("%w: got %q", ErrInvalidLanguageTag, meta.Language)
	}

	if meta.Star < MinStar || meta.Star > MaxStar {
		return util.Errorf("%w: got %d", ErrInvalidStar, meta.Star)
	}

	if !validateBookFormat(meta.Format) {
		return util.Errorf("%w: got %q", ErrInvalidBookFormat, meta.Format)
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

	err = fsutil.WriteFileAtomic(b.root, metaPath, bs)
	if err != nil {
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

type NewSourceOptions struct {
	Format  string
	Comment string
}

// NewSource creates a new plain-text source. Use NewSourceWithOptions when the
// caller knows the source is Markdown or wants to record its provenance.
func (b *Book) NewSource(source io.Reader) (*Source, error) {
	return b.NewSourceWithOptions(source, NewSourceOptions{Format: BookFormatText})
}

// NewSourceWithOptions atomically publishes one complete source folder. A
// failed content or metadata write only leaves a hidden temporary directory,
// which is removed before the call returns.
func (b *Book) NewSourceWithOptions(source io.Reader, options NewSourceOptions) (*Source, error) {
	if err := b.EnsureWritable(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if source == nil {
		source = io.NopCloser(strings.NewReader(""))
	}
	if options.Format == "" {
		options.Format = BookFormatText
	}
	if !validateBookFormat(options.Format) {
		return nil, util.Errorf("%w: got %q", ErrInvalidBookFormat, options.Format)
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

	tempSourcePath := path.Join(b.folderPath, SourcesFolder, "."+sourceID+"-"+randomString(6)+".tmp")
	defer b.root.RemoveAll(tempSourcePath) //nolint:errcheck // best-effort cleanup of unpublished data

	src, err := createSource(b.root, b.logger, tempSourcePath, sourceID, source, options.Format, options.Comment)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := b.root.Rename(tempSourcePath, sourcePath); err != nil {
		return nil, util.Errorf("%w", err)
	}
	src.folderPath = sourcePath

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

// ResolveCurrentSource opens the source a reader should be served, tolerating a
// current_source pointer that no longer resolves. A shelf is edited by hand and
// by sync tools, so the pointer can name a source that was removed or damaged
// outside this build; falling back to the newest surviving source keeps such a
// book readable. This is a read path: it never repairs book.json, because the
// filesystem stays the source of truth and only an explicit write may change it.
func (b *Book) ResolveCurrentSource() (*Source, error) {
	// GetSource("") fails path-segment validation rather than reporting a
	// missing source, so an unset pointer is answered before asking for it.
	if currentID := b.CurrentSource(); currentID != "" {
		source, err := b.GetSource(currentID)
		if err == nil {
			return source, nil
		}
		// A half-removed or unreadable source folder fails in openSource and is
		// not ErrSourceNotFound; for a reader every one of those means the same
		// thing, so the reason is logged and the fallback runs regardless.
		b.logger.Warn("current source is unusable, falling back to the newest source",
			"book", b.folderPath, "current_source", currentID, "error", err)
	}

	sources, err := b.ListSource()
	if err != nil {
		// A book that never had a source has no sources/ folder at all, so this
		// reports fs.ErrNotExist rather than an empty list. That is "nothing to
		// serve", not a broken shelf, so it must not be reported as one.
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

// latestSourceExcluding returns the newest source that is not excludeID, or nil
// when the book has no other source. ListSource sorts by ID, but the choice is
// made explicitly here so it does not silently depend on that.
func (b *Book) latestSourceExcluding(excludeID string) (*Source, error) {
	sources, err := b.ListSource()
	if err != nil {
		// A missing sources/ folder means there is nothing to promote, which is
		// the same answer as an empty list.
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
	if err := validateSourceID(sourceID); err != nil {
		return util.Errorf("%w", err)
	}

	deletingCurrent := sourceID == b.CurrentSource()
	if deletingCurrent {
		// The current-source branch rewrites book.json, so the book-level guard
		// has to run before anything is removed from disk.
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
	// A source written by a newer model may contain metadata this build cannot
	// preserve or even recognize. Treat deleting it as a write and refuse it by
	// the same rule used by content, comment, split, and asset mutations.
	source, err := openSource(b.root, sourcePath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := source.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	// Deleting the active source must not leave current_source dangling, which
	// would make the book unreadable. Hand the pointer over first and only then
	// remove the folder: a replacement created after the removal could reuse the
	// freed ID within the same second, and a failure here leaves the book
	// pointing at a source that exists rather than at one that does not.
	if deletingCurrent {
		if err := b.handOverCurrentSource(sourceID); err != nil {
			return util.Errorf("%w", err)
		}
	}

	err = b.root.RemoveAll(sourcePath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// handOverCurrentSource points current_source at the newest source other than
// leavingID. A book always keeps at least one source — NewBookWith creates an
// empty one — so deleting the last source leaves an empty replacement behind
// instead of a book with nothing to read.
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

// ListSource returns every source of the book, sorted by ID in ascending
// order. The order is this method's own contract: both fsutil.FS
// implementations happen to return sorted directory entries today, but the
// interface does not promise it, and the source list UI and the current-source
// fallback both depend on it.
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
		// NewSourceWithOptions publishes from a hidden *.tmp directory. A crash
		// can leave that unpublished directory behind, but it must never become
		// part of the visible source model or duplicate the embedded final ID.
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

func openBook(rt fsutil.FS, logger logutil.Logger, bookPath string) (*Book, error) {
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

	// A too-new book is deliberately NOT an error here. Failing would make the
	// book vanish from listings (iterateShelfTree), get evicted from the cache
	// (onlyRefreshBooksInCache), 404 from the API, and — worst — become
	// impossible to restore from trash. A visible, explained book beats a book
	// that silently disappears. setMeta is what protects the bytes on disk.
	//
	// This normalization lives here rather than in readBookMeta because that
	// decoder is also used to re-read the persisted cover name, which needs the
	// raw bytes and no warning.
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
		meta:       meta,
		logger:     logger,

		metaStat: *metaStat,
	}, nil
}

// readBookMeta decodes a book's persisted metadata from disk.
func readBookMeta(rt fsutil.FS, bookPath string) (*BookMeta, error) {
	metaFile, err := rt.Open(path.Join(bookPath, BookMetaFile))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta BookMeta
	if err := json.NewDecoder(metaFile).Decode(&meta); err != nil {
		return nil, util.Errorf("%w", err)
	}

	return &meta, nil
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

		metaStat: *metaStat,
	}, nil
}
