package server

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/shelf"
)

// testEPUBTitle is the dc:title inside the archive buildTestEPUB produces.
const testEPUBTitle = "星塵集"

// buildTestEPUB returns a small but complete EPUB 3 archive: two chapters, a
// navigation document, a cover, and enough metadata to exercise every field the
// importer copies onto the book.
func buildTestEPUB(t *testing.T) []byte {
	t.Helper()

	entries := []struct{ name, body string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`},
		{"OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + testEPUBTitle + `</dc:title>
    <dc:creator>林望舒</dc:creator>
    <dc:language>zh-Hant</dc:language>
    <dc:description>一部關於旅途的短篇小說。</dc:description>
    <dc:date>2024-03-01</dc:date>
    <dc:identifier id="pub-id">urn:isbn:9781234567897</dc:identifier>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="cover-img" href="cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="nav"/><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`},
		{"OEBPS/nav.xhtml", `<html xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
  <li><a href="ch1.xhtml">啟程</a></li>
  <li><a href="ch2.xhtml">歸途</a></li>
</ol></nav></body></html>`},
		{"OEBPS/cover.png", string(onePixelPNG())},
		{"OEBPS/ch1.xhtml", `<html><body><h1>第一章</h1><p>他走出了車站。</p></body></html>`},
		{"OEBPS/ch2.xhtml", `<html><body><h1>第二章</h1><p>回程的路上。</p></body></html>`},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", e.name, err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatalf("write zip entry %q: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	return buf.Bytes()
}

// onePixelPNG is a real 1x1 PNG so the cover survives the JPEG conversion the
// import performs when cover_to_jpg is enabled.
func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D,
		0xB0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

func TestEPUBBookTitle(t *testing.T) {
	parsed := &epub.Book{Title: "星塵集"}
	untitled := &epub.Book{}

	tests := []struct {
		name     string
		supplied string
		filename string
		parsed   *epub.Book
		want     string
	}{
		{
			name:     "no supplied title uses the epub title",
			filename: "three-body.epub",
			parsed:   parsed,
			want:     "星塵集",
		},
		{
			// Clients derive a title from the filename when the user typed none,
			// and the book knows its own name better than the file does.
			name:     "filename-derived title loses to the epub title",
			supplied: "three-body",
			filename: "three-body.epub",
			parsed:   parsed,
			want:     "星塵集",
		},
		{
			name:     "full filename also loses to the epub title",
			supplied: "three-body.epub",
			filename: "three-body.epub",
			parsed:   parsed,
			want:     "星塵集",
		},
		{
			name:     "a real user title wins",
			supplied: "我自己取的書名",
			filename: "three-body.epub",
			parsed:   parsed,
			want:     "我自己取的書名",
		},
		{
			name:     "no epub title falls back to the supplied title",
			supplied: "three-body",
			filename: "three-body.epub",
			parsed:   untitled,
			want:     "three-body",
		},
		{
			name:     "nothing at all falls back to the filename",
			filename: "three-body.epub",
			parsed:   untitled,
			want:     "three-body.epub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := epubBookTitle(tt.supplied, tt.filename, tt.parsed); got != tt.want {
				t.Errorf("epubBookTitle(%q, %q) = %q, want %q", tt.supplied, tt.filename, got, tt.want)
			}
		})
	}
}

func TestSplitConfigForRender(t *testing.T) {
	tests := []struct {
		name     string
		rendered epub.Rendered
		want     shelf.SplitConfig
	}{
		{
			name:     "regex is preferred so the reader can show chapter names",
			rendered: epub.Rendered{ChapterRegex: "^## ", ChapterLines: []int{3, 9}},
			want:     shelf.SplitConfig{Type: shelf.SplitTypeRegex, Regex: "^## "},
		},
		{
			name:     "no regex falls back to explicit boundaries",
			rendered: epub.Rendered{ChapterLines: []int{3, 9}},
			want:     shelf.SplitConfig{Type: shelf.SplitTypeBoundary, Boundaries: []int{3, 9}},
		},
		{
			name:     "a single chapter is not split",
			rendered: epub.Rendered{ChapterRegex: "^## ", ChapterLines: []int{3}},
			want:     shelf.SplitConfig{Type: shelf.SplitTypeNone},
		},
		{
			name:     "no chapters is not split",
			rendered: epub.Rendered{},
			want:     shelf.SplitConfig{Type: shelf.SplitTypeNone},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitConfigForRender(tt.rendered)
			if got.Type != tt.want.Type || got.Regex != tt.want.Regex {
				t.Errorf("splitConfigForRender() = %+v, want %+v", got, tt.want)
			}
			if len(got.Boundaries) != len(tt.want.Boundaries) {
				t.Fatalf("Boundaries = %v, want %v", got.Boundaries, tt.want.Boundaries)
			}
			for i := range got.Boundaries {
				if got.Boundaries[i] != tt.want.Boundaries[i] {
					t.Errorf("Boundaries = %v, want %v", got.Boundaries, tt.want.Boundaries)
				}
			}
		})
	}
}

func TestSplitConfigForRenderUsesBoundariesForUntitledChapter(t *testing.T) {
	rendered := epub.Render(&epub.Book{
		Title: "Untitled chapter",
		Chapters: []epub.Chapter{
			{Title: "Chapter One", Text: "First."},
			{Text: "Second."},
		},
	}, epub.Strategy{Preset: epub.PresetMarkdown})

	if rendered.ChapterRegex != "" {
		t.Fatalf("ChapterRegex = %q, want the renderer to disable the incomplete regex", rendered.ChapterRegex)
	}

	got := splitConfigForRender(rendered)
	if got.Type != shelf.SplitTypeBoundary {
		t.Fatalf("split type = %q, want boundary", got.Type)
	}
	if len(got.Boundaries) != 2 || got.Boundaries[0] != rendered.ChapterLines[0] || got.Boundaries[1] != rendered.ChapterLines[1] {
		t.Errorf("boundaries = %v, want rendered chapter lines %v", got.Boundaries, rendered.ChapterLines)
	}
}

func TestParseEPUBDate(t *testing.T) {
	tests := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		{raw: "2024-03-01", want: "2024-03-01", wantOK: true},
		{raw: "2024-03-01T12:30:00Z", want: "2024-03-01", wantOK: true},
		{raw: "2024-03-01T12:30:00", want: "2024-03-01", wantOK: true},
		{raw: "2024-03", want: "2024-03-01", wantOK: true},
		{raw: "2024", want: "2024-01-01", wantOK: true},
		{raw: "  2024-03-01  ", want: "2024-03-01", wantOK: true},
		{raw: "", wantOK: false},
		{raw: "sometime in spring", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := parseEPUBDate(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("parseEPUBDate(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if formatted := time.Time(got).Format(time.DateOnly); formatted != tt.want {
				t.Errorf("parseEPUBDate(%q) = %s, want %s", tt.raw, formatted, tt.want)
			}
		})
	}
}

func TestParseImportStrategy(t *testing.T) {
	fallback := epub.Strategy{Preset: epub.PresetPlain, IncludeDescription: false}

	tests := []struct {
		name    string
		raw     string
		want    epub.Strategy
		wantErr bool
	}{
		{
			name: "empty uses the fallback",
			raw:  "",
			want: fallback,
		},
		{
			name: "whitespace uses the fallback",
			raw:  "   ",
			want: fallback,
		},
		{
			name: "explicit strategy overrides the fallback",
			raw:  `{"preset":"markdown","include_description":true}`,
			want: epub.Strategy{Preset: epub.PresetMarkdown, IncludeDescription: true},
		},
		{name: "unknown preset is rejected", raw: `{"preset":"custom"}`, wantErr: true},
		{name: "missing preset is rejected", raw: `{"include_description":true}`, wantErr: true},
		{name: "unknown field is rejected", raw: `{"preset":"plain","template":"x"}`, wantErr: true},
		{name: "malformed json is rejected", raw: `{`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, message, err := parseImportStrategy(tt.raw, fallback)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseImportStrategy(%q) error = nil, want an error", tt.raw)
				}
				// The message is what the client sees; the error is logged and
				// carries the util.Errorf prefix, which must not leak into it.
				if message == "" {
					t.Fatalf("parseImportStrategy(%q) returned no client message", tt.raw)
				}
				if strings.Contains(message, "plainshelf/") {
					t.Fatalf("message = %q, must not name internal packages", message)
				}
				return
			}
			if message != "" {
				t.Fatalf("parseImportStrategy(%q) message = %q, want empty when accepted", tt.raw, message)
			}
			if err != nil {
				t.Fatalf("parseImportStrategy(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("parseImportStrategy(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}
