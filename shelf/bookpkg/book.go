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
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

/*
{book-folder}/
├─ book.json
├─ CURRENT_SOURCE.txt
├─ cover.(jpg|png|webp)
└─ sources/
   └─ {source-id}
*/

const SourcesFolder = "sources"
const BookMetaFile = "book.json"

// Nothing ever reads either hint file back — current_source in book.json is the
// only authority — so the rename needed no migration: the next hint write drops
// the legacy file, and a shelf never carries both.
const CurrentSourceHintFile = "CURRENT_SOURCE.txt"
const LegacyCurrentSourceHintFile = "CURRENT_VERSION_LOCATION.txt"

// The hint is English rather than a UI locale: i18n lives in the frontend and a
// headless write has no locale to read. The file is disposable, so a future
// locale-aware write could undo this at no cost.
const CurrentSourceHintTemplate = `[PlainShelf hint]
This book currently reads from:
%s

This file is a hint written by PlainShelf, not the source of truth: nothing
reads it back, and current_source in book.json is what the app follows. You
can safely delete it; it is rewritten the next time the current source changes.
`

// BookMetaSchemaVersion is the book.json schema version this build writes.
//
// A book.json with no schema_version predates versioning ("v0"): it is read as
// v1 and normalized in memory, and the version reaches disk only on the next
// write, so opening a library never rewrites it.
//
// A HIGHER schema_version is read best-effort but never written back, so an
// older build cannot clobber a newer one's data. EnsureWritable enforces that.
const BookMetaSchemaVersion = 1

var ErrSourceNotFound = util.NewError("source not found")
var ErrInvalidIdentifierKey = util.NewError("identifier key cannot be empty")

const (
	MinStar = 0
	MaxStar = 5
)

var ErrInvalidStar = util.NewError("star must be between 0 and 5")

// ErrInvalidLanguageTag is not reported for an empty language, which means
// "unknown".
var ErrInvalidLanguageTag = util.NewError("language must be a BCP 47 tag")

// BookMeta.Format decides how the reader renders the book's text; the bytes on
// disk are the same either way, so switching rewrites nothing but book.json.
// The canonical values live in shelfutil, which owns ValidateBookFormat.
const (
	BookFormatText     = shelfutil.BookFormatText
	BookFormatMarkdown = shelfutil.BookFormatMarkdown
)

// ErrInvalidBookFormat is not reported for an empty format: books created
// before the field existed have none.
var ErrInvalidBookFormat = util.NewError(`format must be "txt" or "md"`)

// ErrUnsupportedBookSchemaVersion is book.json specific on purpose:
// sources/{id}/meta.json and trash.json carry their own sentinels, so errors.Is
// can tell which file is too new.
var ErrUnsupportedBookSchemaVersion = util.NewError("book.json schema version is newer than this build supports")

type Book struct {
	logger logutil.Logger
	// root is a ReadFS so that a read path cannot mutate the book by accident;
	// every mutation narrows it back through writeRoot.
	root       fsutil.ReadFS
	folderPath string
	meta       *BookMeta

	metaStat FileStat
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
	// FIXME: a change that preserves both mtime and size is not detected.
	metaPath := path.Join(b.folderPath, BookMetaFile)

	currentMetaStat, err := getFileStat(b.root, metaPath)
	if err != nil {
		// Stale is the safe answer; the caller sees the real error on reopen.
		b.logger.Warn("failed to stat meta file during IsStale check, treating book as stale", "error", err)
		return true
	}

	if !currentMetaStat.Equal(&b.metaStat) {
		return true
	}

	return false
}

func (b *Book) ID() string {
	return b.meta.ID
}

func (b *Book) Title() string {
	return b.meta.Title
}

func (b *Book) PackagePath() string {
	return b.folderPath
}

// CoverETag returns an empty string when there is no cover or the stat fails.
func (b *Book) CoverETag() string {
	if b.meta.Cover == "" {
		return ""
	}
	return shelfutil.FileETag(b.root, path.Join(b.folderPath, b.meta.Cover))
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

// writeRoot asks whether the shelf may be written at all; EnsureWritable asks
// whether this build understands the book well enough to rewrite it. Both
// guards run before a mutation.
func (b *Book) writeRoot() (fsutil.FS, error) {
	root, err := fsutil.Writable(b.root)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	return root, nil
}

// EnsureWritable refuses a book.json newer than this build supports.
//
// Call it before any filesystem mutation, not only before writing book.json: a
// guard that runs last still lets a refused request truncate a cover, delete a
// file, or rename a folder first. It is checked against b.meta — what is
// actually on disk — so a caller-supplied BookMeta cannot bypass it.
func (b *Book) EnsureWritable() error {
	if b.meta.SchemaVersion > BookMetaSchemaVersion {
		return util.Errorf("%w: book.json is schema_version %d, this build writes %d",
			ErrUnsupportedBookSchemaVersion, b.meta.SchemaVersion, BookMetaSchemaVersion)
	}
	return nil
}

func (b *Book) SetCover(imageData []byte, ext string) error {
	// Guard before touching the filesystem: refusing only at SetMeta would
	// already have published the new cover file.
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	coverFilename := "cover" + ext
	coverPath := path.Join(b.folderPath, coverFilename)

	previousCover := b.meta.Cover

	// Write the image before pointing the meta at it, so the meta never
	// references a half-written cover file.
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

	// A different extension leaves the old image behind unreferenced (the API
	// converts uploads to JPEG, so cover.png -> cover.jpg is a normal path).
	// The shelf is meant to be browsable by hand, so don't leave the orphan.
	if previousCover != "" && previousCover != coverFilename {
		b.removeReplacedCover(root, previousCover)
	}

	return nil
}

// removeReplacedCover deletes the replaced cover only if the book still points
// away from it. An overlapping upload can write a new image under the old name
// and point the book back at it, and losing an orphan is cheaper than losing
// the live cover.
func (b *Book) removeReplacedCover(root fsutil.FS, previousCover string) {
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
	if b.meta.Cover == "" {
		return nil
	}

	// Guard before Remove: refusing only at setMeta would leave the cover
	// deleted while book.json still references it.
	if err := b.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := b.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	coverPath := path.Join(b.folderPath, b.meta.Cover)
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

	err = b.writeCurrentSourceHint(sourceID)
	if err != nil {
		b.logger.Warn("failed to write current source hint", "error", err)
	}

	return nil
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

	// A shelf written by an older build still carries the previous name. Drop
	// it here so the two hints never sit side by side, contradicting each other
	// once the current source moves again.
	err = root.Remove(path.Join(b.folderPath, LegacyCurrentSourceHintFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	return nil
}

// GetMeta returns a copy the caller may modify and hand back to SetMeta.
func (b *Book) GetMeta() *BookMeta {
	metaCopy := *b.meta
	metaCopy.Tags = slices.Clone(b.meta.Tags)
	metaCopy.Authors = slices.Clone(b.meta.Authors)
	metaCopy.Identifiers = maps.Clone(b.meta.Identifiers)
	return &metaCopy
}

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

	// The backstop for book.json; callers that touch other files guard earlier.
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
	// Stamped unconditionally, which is what makes the v0 → v1 upgrade lazy.
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

// NewSource creates a plain-text source. Use NewSourceWithOptions when the
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

	// The ID is a second-granularity timestamp, so two sources created within
	// the same second would collide. Bump a numeric suffix until the folder name
	// is free (same scheme as NewBook).
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

	tempSourcePath := path.Join(b.folderPath, SourcesFolder, "."+sourceID+"-"+shelfutil.RandomString(6)+".tmp")
	defer root.RemoveAll(tempSourcePath) //nolint:errcheck // best-effort cleanup of unpublished data

	src, err := createSource(root, b.logger, tempSourcePath, sourceID, source, options.Format, options.Comment)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := root.Rename(tempSourcePath, sourcePath); err != nil {
		return nil, util.Errorf("%w", err)
	}
	src.folderPath = sourcePath

	return src, nil
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

// CurrentSourceCharCount reports 0 when there is no current source or it cannot
// be read: a listing treats an unknown count as absent rather than failing over
// one damaged book.
//
// meta.json is opened directly instead of through GetSource, which stats the
// source folder first: the open below already reports a missing source, and on
// a network mount that stat is another round trip for every book in the shelf.
func (b *Book) CurrentSourceCharCount() int {
	sourceID := b.CurrentSource()
	if err := shelfutil.ValidateSourceID(sourceID); err != nil {
		return 0
	}

	source, err := openSource(b.root, path.Join(b.folderPath, SourcesFolder, sourceID))
	if err != nil {
		return 0
	}

	return source.GetMeta().CharCount
}

// ResolveCurrentSource tolerates a current_source pointer that no longer
// resolves: a shelf edited by hand or by sync tools can point at a source
// removed outside this build, and falling back to the newest surviving source
// keeps the book readable. It never repairs book.json — only an explicit write
// may change the shelf.
func (b *Book) ResolveCurrentSource() (*Source, error) {
	// GetSource("") fails path-segment validation rather than reporting a
	// missing source, so an unset pointer is answered before asking for it.
	if currentID := b.CurrentSource(); currentID != "" {
		source, err := b.GetSource(currentID)
		if err == nil {
			return source, nil
		}
		// A half-removed folder fails in openSource, which is not
		// ErrSourceNotFound; to a reader every failure means the same thing.
		b.logger.Warn("current source is unusable, falling back to the newest source",
			"book", b.folderPath, "current_source", currentID, "error", err)
	}

	sources, err := b.ListSource()
	if err != nil {
		// A book that never had a source has no sources/ folder at all, so this
		// reports fs.ErrNotExist rather than an empty list.
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

// latestSourceExcluding picks the newest explicitly, so it does not silently
// depend on ListSource's ordering.
func (b *Book) latestSourceExcluding(excludeID string) (*Source, error) {
	sources, err := b.ListSource()
	if err != nil {
		// A missing sources/ folder is the same answer as an empty list.
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
	// A source written by a newer build may hold metadata this one cannot even
	// recognize, so deleting it counts as a write.
	source, err := openSource(b.root, sourcePath)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := source.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}

	// Hand the pointer over before removing the folder: a replacement created
	// after the removal could reuse the freed ID within the same second, and a
	// failure here leaves the book pointing at a source that still exists.
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

// handOverCurrentSource points current_source at the newest source other than
// leavingID. A book always keeps at least one source, so deleting the last one
// leaves an empty replacement behind rather than nothing to read.
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

// ListSource sorts by ID ascending. The order is this method's own contract:
// fsutil.ReadFS happens to return sorted entries today but does not promise it,
// and both the source list UI and the current-source fallback depend on it.
func (b *Book) ListSource() ([]*Source, error) {
	sourcesPath := path.Join(b.folderPath, SourcesFolder)

	sourceEntries, err := b.root.ReadDir(sourcesPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var sources []*Source
	for _, entry := range sourceEntries {
		// A stray file from a sync tool must not fail the whole listing.
		if !entry.IsDir() {
			continue
		}

		revID := entry.Name()
		// A crash can leave NewSourceWithOptions' unpublished *.tmp directory
		// behind, which must never enter the visible source model.
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

	// A too-new book is deliberately NOT an error here: failing would drop it
	// from listings, 404 from the API and — worst — make it unrestorable from
	// trash. setMeta is what protects the bytes on disk.
	switch {
	case meta.SchemaVersion > BookMetaSchemaVersion:
		logger.Warn("book.json schema version is newer than this build supports; book is read-only",
			"path", metaPath,
			"schema_version", meta.SchemaVersion,
			"supported", BookMetaSchemaVersion)
	case meta.SchemaVersion < BookMetaSchemaVersion:
		// Missing (pre-v1), zero, or garbage. Normalized in memory only; future
		// versions add real field migration here.
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

// readBookMeta keeps encoding/json/v2's strict defaults — case-sensitive member
// names, duplicate members and invalid UTF-8 rejected — on purpose for a file
// people edit by hand. A failure names the file; see [MalformedMetadataError].
func readBookMeta(rt fsutil.ReadFS, bookPath string) (*BookMeta, error) {
	metaPath := path.Join(bookPath, BookMetaFile)
	metaFile, err := rt.Open(metaPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer metaFile.Close()

	var meta BookMeta
	if err := json.UnmarshalRead(metaFile, &meta); err != nil {
		return nil, util.Errorf("%w", MetadataReadError(metaPath, err))
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

		metaStat: *metaStat,
	}, nil
}
