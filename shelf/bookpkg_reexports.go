package shelf

import "github.com/voilelab/plainshelf/shelf/bookpkg"

type (
	Book             = bookpkg.Book
	BookMeta         = bookpkg.BookMeta
	Source           = bookpkg.Source
	SourceMeta       = bookpkg.SourceMeta
	NewSourceOptions = bookpkg.NewSourceOptions
	PageList         = bookpkg.PageList
	Page             = bookpkg.Page
)

const (
	SourcesFolder             = bookpkg.SourcesFolder
	BookMetaFile              = bookpkg.BookMetaFile
	CurrentSourceHintTemplate = bookpkg.CurrentSourceHintTemplate
	BookMetaSchemaVersion     = bookpkg.BookMetaSchemaVersion
	SourceMetaFile            = bookpkg.SourceMetaFile
	SourceFile                = bookpkg.SourceFile
	SourceAssetsFolder        = bookpkg.SourceAssetsFolder
)

const (
	BookFormatText     = bookpkg.BookFormatText
	BookFormatMarkdown = bookpkg.BookFormatMarkdown
	BookFormatImages   = bookpkg.BookFormatImages
)

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
	ErrMalformedMetadata              = bookpkg.ErrMalformedMetadata
	ErrNotImageSource                 = bookpkg.ErrNotImageSource
	ErrPageListConflict               = bookpkg.ErrPageListConflict
	ErrInvalidPagePosition            = bookpkg.ErrInvalidPagePosition
	ErrInvalidPageList                = bookpkg.ErrInvalidPageList
	ErrInvalidPageDirection           = bookpkg.ErrInvalidPageDirection
)

type MalformedMetadataError = bookpkg.MalformedMetadataError

var IsSupportedImageExt = bookpkg.IsSupportedImageExt
