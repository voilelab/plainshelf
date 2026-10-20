package epub

import (
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

type Preset string

const (
	PresetMarkdown Preset = "markdown"

	PresetPlain Preset = "plain"
)

type Strategy struct {
	Preset Preset `json:"preset" yaml:"preset"`

	IncludeDescription bool `json:"include_description" yaml:"include_description"`

	KeepImages *bool `json:"keep_images,omitempty" yaml:"keep_images,omitempty"`
}

func DefaultStrategy() Strategy {
	return Strategy{Preset: PresetMarkdown, IncludeDescription: true}
}

func (s Strategy) Validate() error {
	switch s.Preset {
	case PresetMarkdown, PresetPlain:
		return nil
	default:
		return util.Errorf("unknown epub import preset: %q", string(s.Preset))
	}
}

func (s Strategy) Normalized() Strategy {
	if s.Preset == "" {
		s.Preset = DefaultStrategy().Preset
	}
	if s.KeepImages == nil {
		s.KeepImages = new(true)
	}
	return s
}

func (s Strategy) keepImages() bool {
	n := s.Normalized()
	return *n.KeepImages && n.Preset != PresetPlain
}

func (s Strategy) ParseOptions() Options {
	return Options{
		MarkdownInline: s.Normalized().Preset == PresetMarkdown,
		KeepImages:     s.keepImages(),
	}
}

func (s Strategy) Format() string {
	if s.Normalized().Preset == PresetPlain {
		return "txt"
	}
	return "md"
}

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

type Rendered struct {
	Text string

	Format string
}

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

type docBuilder struct {
	b strings.Builder
}

func (d *docBuilder) appendBlock(block string) {
	block = strings.Trim(block, "\n")
	if block == "" {
		return
	}

	if d.b.Len() > 0 {
		d.b.WriteString("\n\n")
	}
	d.b.WriteString(block)
}

func (d *docBuilder) String() string {
	return d.b.String()
}

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
