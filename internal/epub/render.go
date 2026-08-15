package epub

import (
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// Preset names a built-in output layout.
type Preset string

const (
	// PresetMarkdown writes Markdown headings and keeps inline emphasis. The
	// book is stored with format "md" so the reader renders it as Markdown.
	PresetMarkdown Preset = "markdown"

	// PresetPlain writes chapter titles as bare lines with no markup. The book
	// is stored with format "txt".
	PresetPlain Preset = "plain"
)

// Strategy is the user-selectable conversion strategy. It is small on purpose:
// the layouts themselves are built in, so a stored strategy is an enum plus a
// flag rather than a template that has to be validated and versioned.
type Strategy struct {
	Preset Preset `json:"preset"`

	// IncludeDescription puts the book description at the top of the text as
	// well as in the book metadata, where it is always recorded.
	IncludeDescription bool `json:"include_description"`

	// KeepImages stores the EPUB's illustrations beside the text and links to
	// them, for presets whose output is Markdown. Nil means "not specified",
	// which Normalized reads as enabled: a pointer rather than a plain bool
	// because a stored strategy written before this field existed must keep
	// illustrations rather than inherit a false zero value.
	KeepImages *bool `json:"keep_images,omitempty"`
}

// DefaultStrategy is what an unconfigured import uses.
func DefaultStrategy() Strategy {
	return Strategy{Preset: PresetMarkdown, IncludeDescription: true}
}

// Validate reports whether the strategy names a known preset.
func (s Strategy) Validate() error {
	switch s.Preset {
	case PresetMarkdown, PresetPlain:
		return nil
	default:
		return util.Errorf("unknown epub import preset: %q", string(s.Preset))
	}
}

// Normalized returns the strategy with an empty preset replaced by the default,
// so callers can accept a partially specified strategy without special cases.
func (s Strategy) Normalized() Strategy {
	if s.Preset == "" {
		s.Preset = DefaultStrategy().Preset
	}
	if s.KeepImages == nil {
		s.KeepImages = new(true)
	}
	return s
}

// keepImages reports whether this strategy stores illustrations. A plain-text
// preset never does: the Markdown link would show up literally in the reader.
func (s Strategy) keepImages() bool {
	n := s.Normalized()
	return *n.KeepImages && n.Preset != PresetPlain
}

// ParseOptions returns the parser options this strategy implies. Inline emphasis
// is only useful when the output is stored as Markdown; in plain text the
// markers would show up literally.
func (s Strategy) ParseOptions() Options {
	return Options{
		MarkdownInline: s.Normalized().Preset == PresetMarkdown,
		KeepImages:     s.keepImages(),
	}
}

// Format is the BookMeta.Format value for books produced by this strategy.
func (s Strategy) Format() string {
	if s.Normalized().Preset == PresetPlain {
		return "txt"
	}
	return "md"
}

// layout is a two-part template. Markdown chapter structure is carried by the
// H2 text itself; no parallel split configuration is derived or persisted.
type layout struct {
	header  string
	chapter string
}

func (s Strategy) layout() layout {
	description := ""
	if s.IncludeDescription {
		description = "\n\n{description}"
	}

	if s.Normalized().Preset == PresetPlain {
		return layout{
			header:  "{book_title}" + description,
			chapter: "{chapter_title}\n\n{chapter_content}",
		}
	}

	return layout{
		header:  "# {book_title}" + description,
		chapter: "## {chapter_title}\n\n{chapter_content}",
	}
}

// Rendered is the import-ready result of applying a strategy to a parsed book.
type Rendered struct {
	// Text is the full book content destined for source.txt.
	Text string

	// Format is the BookMeta.Format value ("md" or "txt").
	Format string
}

// Render lays a parsed book out as text according to strategy.
func Render(book *Book, strategy Strategy) Rendered {
	strategy = strategy.Normalized()
	l := strategy.layout()

	chapterCount := strconv.Itoa(len(book.Chapters))

	var doc docBuilder
	doc.appendBlock(substitute(l.header, map[string]string{
		"book_title":    book.Title,
		"author":        strings.Join(book.Authors, ", "),
		"description":   book.Description,
		"language":      book.Language,
		"chapter_count": chapterCount,
	}))

	for i, chapter := range book.Chapters {
		chapterTitle := strings.TrimSpace(chapter.Title)
		if chapterTitle == "" {
			chapterTitle = "Part " + strconv.Itoa(i+1)
		}
		block := substitute(l.chapter, map[string]string{
			"chapter_title":   chapterTitle,
			"chapter_content": chapter.Text,
			"chapter_index":   strconv.Itoa(i + 1),
			"chapter_count":   chapterCount,
		})
		block = normalizeBlock(block)
		if block == "" {
			continue
		}

		doc.appendBlock(block)
	}

	return Rendered{
		Text:   doc.String(),
		Format: strategy.Format(),
	}
}

// docBuilder assembles blocks separated by exactly one blank line, tracking how
// many lines have been written so chapter start lines are exact.
type docBuilder struct {
	b strings.Builder
}

func (d *docBuilder) appendBlock(block string) {
	block = strings.Trim(block, "\n")
	if block == "" {
		return
	}

	if d.b.Len() > 0 {
		d.write("\n\n")
	}
	d.write(block)
}

func (d *docBuilder) write(s string) {
	d.b.WriteString(s)
}

func (d *docBuilder) String() string {
	return d.b.String()
}

// substitute replaces {placeholder} spans. "{{" is an escape for a literal "{",
// and an unknown placeholder is left untouched rather than silently blanked.
func substitute(tmpl string, vars map[string]string) string {
	var b strings.Builder
	b.Grow(len(tmpl))

	for i := 0; i < len(tmpl); {
		c := tmpl[i]
		if c != '{' {
			b.WriteByte(c)
			i++
			continue
		}

		if i+1 < len(tmpl) && tmpl[i+1] == '{' {
			b.WriteByte('{')
			i += 2
			continue
		}

		end := strings.IndexByte(tmpl[i:], '}')
		if end < 0 {
			b.WriteString(tmpl[i:])
			break
		}
		end += i

		key := tmpl[i+1 : end]
		if value, ok := vars[key]; ok {
			b.WriteString(value)
		} else {
			b.WriteString(tmpl[i : end+1])
		}
		i = end + 1
	}

	return b.String()
}

// normalizeBlock collapses runs of blank lines left behind by empty
// placeholders. Only trailing whitespace is trimmed, so U+3000 paragraph
// indentation survives.
func normalizeBlock(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	pendingBlank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			pendingBlank = true
			continue
		}
		if pendingBlank && len(out) > 0 {
			out = append(out, "")
		}
		pendingBlank = false
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
