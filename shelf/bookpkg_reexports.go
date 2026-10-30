package shelf

import "github.com/voilelab/plainshelf/shelf/bookpkg"

type (
	Book             = bookpkg.Book
	BookMeta         = bookpkg.BookMeta
	Source           = bookpkg.Source
	SourceMeta       = bookpkg.SourceMeta
	NewSourceOptions = bookpkg.NewSourceOptions
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
)

type MalformedMetadataError = bookpkg.MalformedMetadataError

var IsSupportedImageExt = bookpkg.IsSupportedImageExt
