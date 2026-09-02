package shelf

import "github.com/voilelab/plainshelf/shelf/bookpkg"

// The .bookpkg read/write layer lives in shelf/bookpkg so a reader can open one
// book package without compiling in the shelf's file lock, trash, caches and
// scan tree. These aliases keep the symbols call sites name reachable as shelf.X.
//
// bookpkg does not import shelf: the dependency runs one way only, through the
// shared shelf/internal/shelfutil leaf.
//
// Measured (PSW-59): dropping these aliases and writing bookpkg.X at every call
// site rewrites 47 files for 39 new import lines and a net -49 lines, and splits
// one vocabulary in two - shelf.ErrBookNotFound beside bookpkg.ErrSourceNotFound
// in one errors.Is, *shelf.ShelfData beside *bookpkg.Book in one signature. The
// aliases pay for themselves; mirroring symbols nobody names did not, so the ten
// without a shelf.X call site are gone. Add one back when a call site needs it.

// Book package and book/source types. Aliases, so methods and struct fields
// travel with them and a *bookpkg.Book is a *shelf.Book.
type (
	Book             = bookpkg.Book
	BookMeta         = bookpkg.BookMeta
	Source           = bookpkg.Source
	SourceMeta       = bookpkg.SourceMeta
	NewSourceOptions = bookpkg.NewSourceOptions
)

// Book package filenames, folder names and schema versions.
const (
	SourcesFolder             = bookpkg.SourcesFolder
	BookMetaFile              = bookpkg.BookMetaFile
	CurrentSourceHintTemplate = bookpkg.CurrentSourceHintTemplate
	BookMetaSchemaVersion     = bookpkg.BookMetaSchemaVersion
	SourceMetaFile            = bookpkg.SourceMetaFile
	SourceFile                = bookpkg.SourceFile
	SourceAssetsFolder        = bookpkg.SourceAssetsFolder
)

// Metadata value vocabularies.
const (
	BookFormatText     = bookpkg.BookFormatText
	BookFormatMarkdown = bookpkg.BookFormatMarkdown
)

// Sentinel errors raised by the book package folder.
var (
	ErrSourceNotFound                 = bookpkg.ErrSourceNotFound
	ErrInvalidIdentifierKey           = bookpkg.ErrInvalidIdentifierKey
	ErrInvalidStar                    = bookpkg.ErrInvalidStar
	ErrInvalidLanguageTag             = bookpkg.ErrInvalidLanguageTag
	ErrInvalidBookFormat              = bookpkg.ErrInvalidBookFormat
	ErrUnsupportedBookSchemaVersion   = bookpkg.ErrUnsupportedBookSchemaVersion
	ErrUnsupportedSourceSchemaVersion = bookpkg.ErrUnsupportedSourceSchemaVersion
	ErrAssetNotFound                  = bookpkg.ErrAssetNotFound
	ErrInvalidAssetName               = bookpkg.ErrInvalidAssetName
)

// IsSupportedImageExt reports whether an extension names an image a source can
// keep as an asset.
var IsSupportedImageExt = bookpkg.IsSupportedImageExt
