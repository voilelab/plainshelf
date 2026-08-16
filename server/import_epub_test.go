package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/epub"
	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/shelf"
)

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

func TestEPUBRenderUsesStableHeadingForUntitledChapter(t *testing.T) {
	rendered := epub.Render(&epub.Book{
		Title: "Untitled chapter",
		Chapters: []epub.Chapter{
			{Title: "Chapter One", Text: "First."},
			{Text: "Second."},
		},
	}, epub.Strategy{Preset: epub.PresetMarkdown})

	if !strings.Contains(rendered.Text, "## Part 2") {
		t.Fatalf("rendered text = %q, want stable H2 for untitled chapter", rendered.Text)
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

func TestEPUBImportComment(t *testing.T) {
	tests := []struct {
		name string
		book *epub.Book
		want string
	}{
		{
			// An import that lost nothing leaves no note behind, so the comment
			// stays available for anything else and book.json stays quiet.
			name: "nothing dropped leaves no comment",
			book: &epub.Book{},
			want: "",
		},
		{
			name: "one image reads as singular",
			book: &epub.Book{DroppedImages: 1},
			want: "Converted from EPUB. 1 embedded image was dropped.",
		},
		{
			name: "several images read as plural",
			book: &epub.Book{DroppedImages: 4},
			want: "Converted from EPUB. 4 embedded images were dropped.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := epubImportComment(tt.book); got != tt.want {
				t.Errorf("epubImportComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestImportEPUBRecordsDroppedImages pins the whole path: the converter counts
// the illustrations, the importer writes the note onto the source it created,
// and the sources endpoint hands it back for the book detail view to show.
func TestImportEPUBRecordsDroppedImages(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
		want    string
	}{
		{
			// Both plates are stored beside the text now, so an illustrated
			// EPUB loses nothing and leaves no note.
			archive: testutil.BuildIllustratedTestEPUB(t),
			want:    "",
		},
		{
			// A format the shelf cannot serve is still a loss, and is still
			// what the note is for.
			name:    "unstorable illustration is still reported",
			archive: testutil.BuildUnstorableImageTestEPUB(t),
			want:    "Converted from EPUB. 1 embedded image was dropped.",
		},
		{
			// The cover is kept, so an EPUB whose only image is the cover has
			// nothing to report.
			name:    "epub without illustrations records nothing",
			archive: testutil.BuildTestEPUB(t),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newAPITestEnv(t)
			imported := importFileBook(t, env, "book.epub", "application/epub+zip", string(tt.archive))

			if imported.Meta == nil || imported.Meta.CurrentSource == "" {
				t.Fatal("imported book has no current source")
			}

			path := "/api/shelves/default_shelf/books/" + imported.Meta.ID +
				"/sources/" + imported.Meta.CurrentSource
			rec := env.do(httptest.NewRequest(http.MethodGet, path, nil))
			assertStatus(t, rec, http.StatusOK)

			source := decodeJSON[shelf.SourceMeta](t, rec)
			if source.Comment != tt.want {
				t.Errorf("source comment = %q, want %q", source.Comment, tt.want)
			}
		})
	}
}

// The illustrations an EPUB carried must land on the shelf, be referenced by
// the text, and come back through the asset route. This is the first test that
// exercises storage, conversion and serving together; each side alone can pass
// while the names they agree on have drifted apart.
func TestImportEPUBStoresIllustrationsAsAssets(t *testing.T) {
	env := newAPITestEnv(t)
	imported := importFileBook(t, env, "book.epub", "application/epub+zip", string(testutil.BuildIllustratedTestEPUB(t)))

	if imported.Meta == nil || imported.Meta.CurrentSource == "" {
		t.Fatal("imported book has no current source")
	}
	base := "/api/shelves/default_shelf/books/" + imported.Meta.ID + "/sources/" + imported.Meta.CurrentSource

	rec := env.do(httptest.NewRequest(http.MethodGet, base+"/content", nil))
	assertStatus(t, rec, http.StatusOK)
	content := rec.Body.String()

	// Both plates are stored, numbered in the order the spine reached them.
	for _, name := range []string{"img-0001.png", "img-0002.png"} {
		link := "![](" + shelf.SourceAssetsFolder + "/" + name + ")"
		if !strings.Contains(content, link) {
			t.Fatalf("converted text does not contain %q:\n%s", link, content)
		}

		rec = env.do(httptest.NewRequest(http.MethodGet, base+"/assets/"+name, nil))
		assertStatus(t, rec, http.StatusOK)
		if got := rec.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("asset %s Content-Type = %q, want image/png", name, got)
		}
		if !bytes.Equal(rec.Body.Bytes(), testutil.OnePixelPNG()) {
			t.Errorf("asset %s bytes do not match the archive entry", name)
		}
	}

	// The book is Markdown, which is the only format whose reader renders the
	// link that was just written into the text.
	if imported.Meta.Format != "md" {
		t.Errorf("imported format = %q, want md", imported.Meta.Format)
	}
}

// epub names its stored images without importing shelf, so the two have to be
// checked against each other: a name shelf refuses would be referenced by the
// text and never load, and an image written outside SourceAssetsFolder would
// not be found at all.
func TestEPUBImageNamingAgreesWithShelf(t *testing.T) {
	if epub.ImageFolder != shelf.SourceAssetsFolder {
		t.Fatalf("epub.ImageFolder = %q, shelf.SourceAssetsFolder = %q", epub.ImageFolder, shelf.SourceAssetsFolder)
	}

	archive := testutil.BuildIllustratedTestEPUB(t)
	parsed, err := epub.Parse(bytes.NewReader(archive), int64(len(archive)), epub.DefaultStrategy().ParseOptions())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(parsed.Images) == 0 {
		t.Fatal("the illustrated fixture kept no images, so this proves nothing")
	}

	for _, image := range parsed.Images {
		if !shelf.IsSupportedImageExt(path.Ext(image.Name)) {
			t.Errorf("epub stored %q, which shelf will not serve", image.Name)
		}
	}
}

// A client written before keep_images existed submits a strategy without it.
// The import dialog does exactly that on every EPUB upload, so reading the
// absent field as "use the built-in default" would make the configured setting
// unreachable from the browser.
func TestImportEPUBPerRequestStrategyInheritsKeepImages(t *testing.T) {
	off := false
	fallback := epub.Strategy{Preset: epub.PresetMarkdown, KeepImages: &off}

	got, _, err := parseImportStrategy(`{"preset":"markdown","include_description":true}`, fallback)
	if err != nil {
		t.Fatalf("parseImportStrategy: %v", err)
	}
	if got.KeepImages == nil || *got.KeepImages {
		t.Fatalf("keep_images = %v, want the configured false", got.KeepImages)
	}

	// An explicit value still wins over the fallback.
	got, _, err = parseImportStrategy(`{"preset":"markdown","keep_images":true}`, fallback)
	if err != nil {
		t.Fatalf("parseImportStrategy with explicit keep_images: %v", err)
	}
	if got.KeepImages == nil || !*got.KeepImages {
		t.Fatalf("explicit keep_images = %v, want true", got.KeepImages)
	}

	// An empty strategy field means "use fallback" and always did.
	got, _, err = parseImportStrategy("", fallback)
	if err != nil {
		t.Fatalf("parseImportStrategy with no strategy: %v", err)
	}
	if got.KeepImages == nil || *got.KeepImages {
		t.Fatalf("fallback keep_images = %v, want false", got.KeepImages)
	}
}

// The end-to-end version of the same thing: with the setting off, an import
// that submits a strategy of its own must still store no illustrations.
func TestImportEPUBHonoursKeepImagesOff(t *testing.T) {
	env := newAPITestEnv(t)

	rec := env.do(httptest.NewRequest(http.MethodPost, "/api/setting/epub_import_strategy",
		strings.NewReader(`{"preset":"markdown","include_description":true,"keep_images":false}`)))
	assertStatus(t, rec, http.StatusNoContent)

	imported := importEPUBWithStrategy(t, env, "book.epub", string(testutil.BuildIllustratedTestEPUB(t)),
		`{"preset":"markdown","include_description":true}`)

	base := "/api/shelves/default_shelf/books/" + imported.Meta.ID + "/sources/" + imported.Meta.CurrentSource
	rec = env.do(httptest.NewRequest(http.MethodGet, base+"/content", nil))
	assertStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "![") {
		t.Fatalf("images were stored despite keep_images=false:\n%s", rec.Body.String())
	}

	rec = env.do(httptest.NewRequest(http.MethodGet, base+"/assets/img-0001.png", nil))
	assertStatus(t, rec, http.StatusNotFound)
}
