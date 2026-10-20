package server

import (
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

type epubInputError struct {
	cause error
}

func (e *epubInputError) Error() string { return e.cause.Error() }
func (e *epubInputError) Unwrap() error { return e.cause }

func isEPUBInputError(err error) bool {
	_, ok := errors.AsType[*epubInputError](err)
	return ok
}

func parseImportStrategy(raw string, fallback epub.Strategy) (epub.Strategy, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, "", nil
	}

	var strategy epub.Strategy
	if err := decodeRequestJSON(strings.NewReader(raw), &strategy, false); err != nil {
		return epub.Strategy{}, "invalid strategy field", util.Errorf("%w", err)
	}
	if err := strategy.Validate(); err != nil {
		return epub.Strategy{}, fmt.Sprintf("unsupported epub import preset: %q", strategy.Preset),
			util.Errorf("%w", err)
	}

	if strategy.KeepImages == nil {
		strategy.KeepImages = fallback.KeepImages
	}

	return strategy, "", nil
}

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

func (h *importHandlers) initEPUBBook(parsed *epub.Book, rendered epub.Rendered) func(*shelf.Book) error {
	return func(book *shelf.Book) error {
		source, err := book.NewSourceWithOptions(strings.NewReader(rendered.Text), shelf.NewSourceOptions{Format: rendered.Format})
		if err != nil {
			return util.Errorf("%w", err)
		}
		h.writeEPUBImages(source, parsed)
		if err := book.SetCurrentSource(source.ID()); err != nil {
			return util.Errorf("%w", err)
		}
		if comment := epubImportComment(parsed); comment != "" {
			if err := source.UpdateComment(comment); err != nil {
				return util.Errorf("%w", err)
			}
		}

		h.setEPUBCover(book, parsed)

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

func epubImportComment(parsed *epub.Book) string {
	switch n := parsed.DroppedImages; {
	case n <= 0:
		return ""
	case n == 1:
		return "Converted from EPUB. 1 embedded image was dropped."
	default:
		return fmt.Sprintf("Converted from EPUB. %d embedded images were dropped.", n)
	}
}

func (h *importHandlers) writeEPUBImages(source *shelf.Source, parsed *epub.Book) {
	for _, image := range parsed.Images {
		if err := source.WriteAsset(image.Name, image.Data); err != nil {
			h.Error("failed to store epub illustration", "error", err, "asset", image.Name)
		}
	}
}

func (h *importHandlers) setEPUBCover(book *shelf.Book, parsed *epub.Book) {
	if len(parsed.Cover) == 0 {
		return
	}

	data := parsed.Cover
	ext := parsed.CoverExt

	if h.settings.coverToJPG() || !shelf.IsSupportedImageExt(ext) {
		converted, err := imgutil.AnyToJPG(data)
		if err != nil {
			h.Error("failed to convert epub cover to JPEG", "error", err)
			return
		}
		data, ext = converted, ".jpg"
	}

	if err := book.SetCover(data, ext); err != nil {
		h.Error("failed to store epub cover", "error", err)
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

func (h *importHandlers) importEPUB(
	shelfData *shelf.ShelfData,
	r io.ReaderAt,
	size int64,
	filename string,
	suppliedTitle string,
	folderParts shelf.FolderPath,
	strategy epub.Strategy,
) (*shelf.Book, error) {

	parsed, err := epub.Parse(r, size, strategy.ParseOptions())
	if err != nil {
		return nil, &epubInputError{cause: err}
	}

	rendered := epub.Render(parsed, strategy)
	title := epubBookTitle(suppliedTitle, filename, parsed)

	book, err := shelfData.NewBookWith(folderParts, title, h.initEPUBBook(parsed, rendered))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return book, nil
}
