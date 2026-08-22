package shelf

import "github.com/voilelab/plainshelf/shelf/bookpkg"

// The .bookpkg read/write layer lives in shelf/bookpkg so a reader can open one
// book package without compiling in the shelf's file lock, trash, caches and
// scan tree. These aliases keep every symbol reachable as shelf.X, so server,
// desktop, cmd and the shelf package's own code are unchanged by the split.
//
// bookpkg does not import shelf: the dependency runs one way only, through the
// shared shelf/internal/shelfutil leaf.

// Book package and book/source types. Aliases, so methods and struct fields
// travel with them and a *bookpkg.Book is a *shelf.Book.
type (
	Book             = bookpkg.Book
	BookMeta         = bookpkg.BookMeta
	Source           = bookpkg.Source
	SourceMeta       = bookpkg.SourceMeta
	NewSourceOptions = bookpkg.NewSourceOptions
	Asset            = bookpkg.Asset
	BookPackage      = bookpkg.BookPackage
	FileStat         = bookpkg.FileStat
	SplitConfig      = bookpkg.SplitConfig
	SplitType        = bookpkg.SplitType
)

// Book package filenames, folder names and schema versions.
const (
	SourcesFolder               = bookpkg.SourcesFolder
	BookMetaFile                = bookpkg.BookMetaFile
	CurrentSourceHintFile       = bookpkg.CurrentSourceHintFile
	LegacyCurrentSourceHintFile = bookpkg.LegacyCurrentSourceHintFile
	CurrentSourceHintTemplate   = bookpkg.CurrentSourceHintTemplate
	BookMetaSchemaVersion       = bookpkg.BookMetaSchemaVersion
	SourceMetaFile              = bookpkg.SourceMetaFile
	SourceFile                  = bookpkg.SourceFile
	SourceMetaSchemaVersion     = bookpkg.SourceMetaSchemaVersion
	SourceAssetsFolder          = bookpkg.SourceAssetsFolder
)

// Metadata value vocabularies.
const (
	MinStar            = bookpkg.MinStar
	MaxStar            = bookpkg.MaxStar
	BookFormatText     = bookpkg.BookFormatText
	BookFormatMarkdown = bookpkg.BookFormatMarkdown

	SplitTypeNone      = bookpkg.SplitTypeNone
	SplitTypeLineCount = bookpkg.SplitTypeLineCount
	SplitTypeRegex     = bookpkg.SplitTypeRegex
	SplitTypeBoundary  = bookpkg.SplitTypeBoundary
)

// Sentinel errors raised by the book package layer.
var (
	ErrSourceNotFound                 = bookpkg.ErrSourceNotFound
	ErrInvalidIdentifierKey           = bookpkg.ErrInvalidIdentifierKey
	ErrInvalidStar                    = bookpkg.ErrInvalidStar
	ErrInvalidLanguageTag             = bookpkg.ErrInvalidLanguageTag
	ErrInvalidBookFormat              = bookpkg.ErrInvalidBookFormat
	ErrUnsupportedBookSchemaVersion   = bookpkg.ErrUnsupportedBookSchemaVersion
	ErrUnsupportedSourceSchemaVersion = bookpkg.ErrUnsupportedSourceSchemaVersion
	ErrNotBookPackage                 = bookpkg.ErrNotBookPackage
	ErrAssetNotFound                  = bookpkg.ErrAssetNotFound
	ErrInvalidAssetName               = bookpkg.ErrInvalidAssetName
	ErrSourceNotLegacy                = bookpkg.ErrSourceNotLegacy
)

// Book package entry points.
var (
	OpenBookPackage     = bookpkg.OpenBookPackage
	IsSupportedImageExt = bookpkg.IsSupportedImageExt
)
