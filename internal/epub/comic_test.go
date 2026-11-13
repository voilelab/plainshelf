package epub

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
)

func comicPageWidths(t *testing.T, comic *Comic) []int {
	t.Helper()
	var widths []int
	for _, page := range comic.Pages {
		data, err := page.Read()
		if err != nil {
			t.Fatal(err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		widths = append(widths, config.Width)
	}
	return widths
}

func parseComicEntries(t *testing.T, entries []zipEntry) (*Comic, error) {
	t.Helper()
	archive := buildArchive(t, entries)
	return ParseComic(bytes.NewReader(archive), int64(len(archive)))
}

func comicEntries(t *testing.T, spineAttrs string, docs ...string) []zipEntry {
	t.Helper()
	entries := []zipEntry{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", containerXML},
		{"OEBPS/p1.png", string(testutil.PNGOfWidth(t, 1))},
		{"OEBPS/p2.png", string(testutil.PNGOfWidth(t, 2))},
		{"OEBPS/scan.svg", "<svg/>"},
	}
	manifest, spine := "", ""
	for i, doc := range docs {
		id := string(rune('a' + i))
		manifest += `<item id="` + id + `" href="` + id + `.xhtml" media-type="application/xhtml+xml"/>`
		spine += `<itemref idref="` + id + `"/>`
		entries = append(entries, zipEntry{"OEBPS/" + id + ".xhtml", "<html><body>" + doc + "</body></html>"})
	}
	return append(entries, zipEntry{"OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata>
  <manifest>` + manifest + `</manifest>
  <spine ` + spineAttrs + `>` + spine + `</spine>
</package>`})
}

func TestParseComicCalibreLayout(t *testing.T) {
	archive := testutil.BuildComicTestEPUB(t, 3)
	comic, err := ParseComic(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if got := comicPageWidths(t, comic); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("page widths = %v, want [1 2 3] without the cover pages", got)
	}
	if comic.Title != testutil.ComicEPUBTitle || strings.Join(comic.Authors, ",") != "Comic Author" ||
		comic.Language != "zh-Hant" || comic.Description != "Comic summary" || comic.Identifiers["isbn"] != "urn:isbn:9781234567897" {
		t.Fatalf("metadata = %+v", comic)
	}
	if comic.Direction != "" {
		t.Fatalf("direction = %q, want left to right", comic.Direction)
	}
	for _, page := range comic.Pages {
		if !strings.HasSuffix(page.Name, ".png") {
			t.Fatalf("page name %q lost its extension", page.Name)
		}
	}
}

func TestParseComicPageOrderAndDirection(t *testing.T) {
	comic, err := parseComicEntries(t, comicEntries(t, `page-progression-direction="rtl"`,
		`<img src="p2.png"/>`, `<p> </p>`, `<div><img src="p1.png"/><img src="p2.png"/></div>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := comicPageWidths(t, comic); len(got) != 2 || got[0] != 2 || got[1] != 1 {
		t.Fatalf("page widths = %v, want spine order [2 1] with repeats dropped", got)
	}
	if comic.Direction != "rtl" {
		t.Fatalf("direction = %q, want rtl", comic.Direction)
	}
}

func TestParseComicCoverOnlyBookKeepsCover(t *testing.T) {
	entries := comicEntries(t, "", `<img src="p1.png"/>`)
	entries[len(entries)-1].body = strings.Replace(entries[len(entries)-1].body, "</package>",
		`<guide><reference type="cover" href="a.xhtml"/></guide></package>`, 1)
	comic, err := parseComicEntries(t, entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := comicPageWidths(t, comic); len(got) != 1 || got[0] != 1 {
		t.Fatalf("page widths = %v, want the cover page kept", got)
	}
}

func TestParseComicRejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"text page", `<img src="p1.png"/><p>第一章</p>`, `"OEBPS/a.xhtml" contains text`},
		{"svg page", `<img src="scan.svg"/>`, "is not a .jpg"},
		{"missing image", `<img src="gone.png"/>`, "missing image"},
		{"no images", `<div></div>`, "no page images"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseComicEntries(t, comicEntries(t, "", tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestStrategyImagesIsNotADefault(t *testing.T) {
	if err := (Strategy{Preset: PresetImages}).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want images accepted for one import", err)
	}
	if err := (Strategy{Preset: PresetImages}).ValidateDefault(); err == nil {
		t.Fatal("ValidateDefault() accepted images as the default preset")
	}
	if err := (Strategy{Preset: PresetPlain}).ValidateDefault(); err != nil {
		t.Fatalf("ValidateDefault() = %v for plain", err)
	}
}
