package epub

import (
	"regexp"
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
	return s
}

// ParseOptions returns the parser options this strategy implies. Inline emphasis
// is only useful when the output is stored as Markdown; in plain text the
// markers would show up literally.
func (s Strategy) ParseOptions() Options {
	return Options{MarkdownInline: s.Normalized().Preset == PresetMarkdown}
}

// Format is the BookMeta.Format value for books produced by this strategy.
func (s Strategy) Format() string {
	if s.Normalized().Preset == PresetPlain {
		return "txt"
	}
	return "md"
}

// layout is a two-part template. There is deliberately no loop construct: with
// the chapter body as its own template, chapter start lines are known exactly
// while rendering, and the chapter heading prefix is known statically, which is
// what makes the split configuration derivable.
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

	// ChapterRegex splits Text into chapters by matching each chapter's heading
	// line. It is empty when the layout writes chapter titles without a literal
	// prefix, because then no pattern can distinguish a title from prose.
	ChapterRegex string

	// ChapterLines holds the 1-based line number each chapter starts on. It is
	// always populated, and is the fallback when ChapterRegex is empty.
	ChapterLines []int
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

	lines := make([]int, 0, len(book.Chapters))
	for i, chapter := range book.Chapters {
		block := substitute(l.chapter, map[string]string{
			"chapter_title":   chapter.Title,
			"chapter_content": chapter.Text,
			"chapter_index":   strconv.Itoa(i + 1),
			"chapter_count":   chapterCount,
		})
		block = normalizeBlock(block)
		if block == "" {
			continue
		}

		lines = append(lines, doc.nextBlockLine())
		doc.appendBlock(block)
	}

	text := doc.String()
	chapterRegex := chapterHeadingRegex(l.chapter)
	if !chapterRegexMatchesLines(text, chapterRegex, lines) {
		chapterRegex = ""
	}

	return Rendered{
		Text:         text,
		Format:       strategy.Format(),
		ChapterRegex: chapterRegex,
		ChapterLines: lines,
	}
}

// chapterRegexMatchesLines reports whether the generated heading regex can
// split at every exact chapter start. Normalization can remove a heading's
// trailing space when its title is empty, so a syntactically valid template
// regex is not necessarily valid for every rendered chapter.
func chapterRegexMatchesLines(text, pattern string, chapterLines []int) bool {
	if pattern == "" {
		return false
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	lines := strings.Split(text, "\n")
	for _, lineNo := range chapterLines {
		if lineNo < 1 || lineNo > len(lines) || !re.MatchString(lines[lineNo-1]) {
			return false
		}
	}
	return true
}

// chapterHeadingRegex builds a pattern matching the literal text a chapter
// template writes before its title. An empty prefix yields an empty pattern:
// with nothing to anchor on, any pattern would match prose as readily as a
// heading.
func chapterHeadingRegex(chapterTemplate string) string {
	idx := strings.Index(chapterTemplate, "{chapter_title}")
	if idx <= 0 {
		return ""
	}

	prefix := chapterTemplate[:idx]
	if strings.TrimSpace(prefix) == "" || strings.Contains(prefix, "\n") {
		return ""
	}

	return "^" + regexp.QuoteMeta(prefix)
}

// docBuilder assembles blocks separated by exactly one blank line, tracking how
// many lines have been written so chapter start lines are exact.
type docBuilder struct {
	b     strings.Builder
	lines int
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
	d.lines += strings.Count(s, "\n")
}

// nextBlockLine is the 1-based line number the next appended block will start
// on. Appending inserts a blank line first, which advances two lines.
func (d *docBuilder) nextBlockLine() int {
	if d.b.Len() == 0 {
		return 1
	}
	return d.lines + 3
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
