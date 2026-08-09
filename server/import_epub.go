package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/imgutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

// coverExtensions are the cover file extensions the read path serves with a
// correct content type. An EPUB cover in any other format is converted to JPEG.
var coverExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// epubInputError marks failures caused by the uploaded archive. Storage and
// other operational failures remain ordinary errors so the HTTP layer can
// report them as server errors without exposing internal details.
type epubInputError struct {
	cause error
}

func (e *epubInputError) Error() string { return e.cause.Error() }
func (e *epubInputError) Unwrap() error { return e.cause }

func isEPUBInputError(err error) bool {
	var inputErr *epubInputError
	return errors.As(err, &inputErr)
}

// parseImportStrategy reads the optional per-import strategy field. An absent or
// empty field means "use fallback", which is how a client that knows nothing
// about strategies keeps working.
//
// Like validateImportFileHeader it returns the client message separately from
// the error, so the rejection can be logged with its cause without that cause
// reaching the response.
func parseImportStrategy(raw string, fallback epub.Strategy) (epub.Strategy, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, "", nil
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()

	var strategy epub.Strategy
	if err := dec.Decode(&strategy); err != nil {
		return epub.Strategy{}, "invalid strategy field", util.Errorf("%w", err)
	}
	if err := strategy.Validate(); err != nil {
		// Name the preset the client sent rather than repeating what epub said
		// about it, matching how the setting route words the same rejection.
		return epub.Strategy{}, fmt.Sprintf("unsupported epub import preset: %q", strategy.Preset),
			util.Errorf("%w", err)
	}

	return strategy, "", nil
}

// epubBookTitle picks the title for an imported EPUB.
//
// A supplied title wins, except when it is just the uploaded filename: clients
// derive a title from the filename when the user did not type one, and the
// book's own dc:title is better than "three-body.epub" every time.
func epubBookTitle(supplied, filename string, parsed *epub.Book) string {
	supplied = strings.TrimSpace(supplied)
	fromFilename := supplied == "" ||
		strings.EqualFold(supplied, filename) ||
		strings.EqualFold(supplied, strings.TrimSuffix(filename, ".epub")) ||
		strings.EqualFold(supplied, strings.TrimSuffix(filename, ".EPUB"))

	if !fromFilename {
		return supplied
	}
	if title := strings.TrimSpace(parsed.Title); title != "" {
		return title
	}
	if supplied != "" {
		return supplied
	}

	return filename
}

// splitConfigForRender turns the rendered chapter positions into the source's
// split configuration. The regex form is preferred because the reader derives
// section titles from it; boundaries are exact but render as "Part N".
func splitConfigForRender(rendered epub.Rendered) shelf.SplitConfig {
	if len(rendered.ChapterLines) < 2 {
		// A single chapter is the whole book; splitting it gains nothing.
		return shelf.SplitConfig{Type: shelf.SplitTypeNone}
	}
	if rendered.ChapterRegex != "" {
		return shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: rendered.ChapterRegex}
	}

	return shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: rendered.ChapterLines}
}

// initEPUBBook fills a freshly staged book with the converted EPUB.
//
// It runs inside NewBookWith while the exclusive shelf lock is held, so it only
// writes; parsing and rendering happen before the book is created.
func (app *App) initEPUBBook(parsed *epub.Book, rendered epub.Rendered) func(*shelf.Book) error {
	return func(book *shelf.Book) error {
		source, err := book.NewSource(strings.NewReader(rendered.Text))
		if err != nil {
			return util.Errorf("%w", err)
		}
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return util.Errorf("%w", err)
		}
		if err := source.UpdateSplitConfig(splitConfigForRender(rendered)); err != nil {
			return util.Errorf("%w", err)
		}

		app.setEPUBCover(book, parsed)

		meta := book.GetMeta()
		meta.Format = rendered.Format
		meta.Authors = parsed.Authors
		meta.Comments = parsed.Description
		meta.Identifiers = parsed.Identifiers

		meta.Language = strings.TrimSpace(parsed.Language)
		if meta.Language == "" {
			meta.Language = detectBookLang(book)
		}

		if published, ok := parseEPUBDate(parsed.PublishedAt); ok {
			meta.PublishedAt = published
		}

		return book.SetMeta(meta)
	}
}

// setEPUBCover stores the EPUB's cover image if it has one. A cover that cannot
// be stored is logged and skipped: a book without a cover is a complete import,
// while failing here would discard the whole book.
func (app *App) setEPUBCover(book *shelf.Book, parsed *epub.Book) {
	if len(parsed.Cover) == 0 {
		return
	}

	data := parsed.Cover
	ext := parsed.CoverExt

	if app.coverToJPG() || !coverExtensions[ext] {
		converted, err := imgutil.AnyToJPG(data)
		if err != nil {
			app.Error("failed to convert epub cover to JPEG", "error", err)
			return
		}
		data, ext = converted, ".jpg"
	}

	if err := book.SetCover(data, ext); err != nil {
		app.Error("failed to store epub cover", "error", err)
	}
}

// epubDateLayouts covers what dc:date actually contains in the wild. EPUB 3
// points at the W3C date-time profile, EPUB 2 at unrestricted ISO 8601, and
// both are routinely ignored.
var epubDateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
	"2006-01",
	"2006",
}

func parseEPUBDate(raw string) (util.JSONDate, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return util.JSONDate{}, false
	}

	for _, layout := range epubDateLayouts {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		y, m, d := parsed.Date()
		return util.JSONDate(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)), true
	}

	return util.JSONDate{}, false
}

// importEPUB parses, renders and stores an EPUB as a new book. The reader must
// also be an io.ReaderAt because zip access is random; size is the archive
// length.
func (app *App) importEPUB(
	shelfData *shelf.ShelfData,
	r io.ReaderAt,
	size int64,
	filename string,
	suppliedTitle string,
	layerParts shelf.Layers,
	strategy epub.Strategy,
) (*shelf.Book, error) {
	// Parse and render before creating the book: the initializer below runs
	// while the exclusive shelf lock is held.
	parsed, err := epub.Parse(r, size, strategy.ParseOptions())
	if err != nil {
		return nil, &epubInputError{cause: err}
	}

	rendered := epub.Render(parsed, strategy)
	title := epubBookTitle(suppliedTitle, filename, parsed)

	book, err := shelfData.NewBookWith(layerParts, title, app.initEPUBBook(parsed, rendered))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return book, nil
}
