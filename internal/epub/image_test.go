package epub

import (
	"strings"
	"testing"
)

// illustratedEntries is an EPUB whose chapters carry artwork: one plate used in
// both chapters, one SVG-wrapped image, and one format the shelf cannot serve.
func illustratedEntries() []zipEntry {
	return []zipEntry{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Illustrated</dc:title>
    <dc:identifier id="pub-id">urn:isbn:9781234567897</dc:identifier>
  </metadata>
  <manifest>
    <item id="cover-img" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
  </spine>
</package>`},
		{"OEBPS/images/cover.png", "cover-bytes"},
		{"OEBPS/images/plate.png", "plate-bytes"},
		{"OEBPS/images/diagram.svg", "<svg/>"},
		{"OEBPS/images/scan.tiff", "tiff-bytes"},
		{"OEBPS/text/ch1.xhtml", `<html><body>
  <h1>Chapter One</h1>
  <p>Before the plate.</p>
  <p><img src="../images/plate.png" alt="A map of the province"/></p>
  <p>After the plate.</p>
</body></html>`},
		{"OEBPS/text/ch2.xhtml", `<html><body>
  <h1>Chapter Two</h1>
  <p>The same plate again.</p>
  <p><img src="../images/plate.png" alt="A map of the province"/></p>
  <p><img src="../images/scan.tiff" alt="An unsupported scan"/></p>
  <svg xmlns="http://www.w3.org/2000/svg"><image href="../images/diagram.svg"/></svg>
</body></html>`},
	}
}

func parseIllustrated(t *testing.T, keep bool) *Book {
	t.Helper()

	book, err := parseArchive(t, buildArchive(t, illustratedEntries()), Options{
		MarkdownInline: true,
		KeepImages:     keep,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return book
}

func TestKeepImagesStoresAndLinksIllustrations(t *testing.T) {
	book := parseIllustrated(t, true)

	if len(book.Images) != 1 {
		t.Fatalf("kept %d images, want 1: %+v", len(book.Images), book.Images)
	}
	if got := book.Images[0].Name; got != "img-0001.png" {
		t.Errorf("image name = %q, want img-0001.png", got)
	}
	if got := string(book.Images[0].Data); got != "plate-bytes" {
		t.Errorf("image data = %q, want plate-bytes", got)
	}

	// The link sits on its own line, which is what the reader turns into a
	// figure, and the prose around it is untouched.
	want := "![A map of the province](assets/img-0001.png)"
	lines := strings.Split(book.Chapters[0].Text, "\n")
	var found bool
	for _, line := range lines {
		if strings.TrimSpace(line) == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("chapter 1 has no line %q:\n%s", want, book.Chapters[0].Text)
	}
	if !strings.Contains(book.Chapters[0].Text, "Before the plate.") ||
		!strings.Contains(book.Chapters[0].Text, "After the plate.") {
		t.Errorf("chapter 1 lost prose around the image:\n%s", book.Chapters[0].Text)
	}
}

// The same file used twice is stored once and both chapters point at it.
func TestKeepImagesDeduplicatesAcrossChapters(t *testing.T) {
	book := parseIllustrated(t, true)

	for i, chapter := range book.Chapters {
		if !strings.Contains(chapter.Text, "(assets/img-0001.png)") {
			t.Errorf("chapter %d does not reference the shared plate:\n%s", i+1, chapter.Text)
		}
	}
	if len(book.Images) != 1 {
		t.Fatalf("stored %d copies of one image, want 1", len(book.Images))
	}
}

// Artwork that cannot be kept still has to be reported, or the note the import
// leaves behind would understate the loss.
func TestKeepImagesStillCountsWhatItCannotStore(t *testing.T) {
	book := parseIllustrated(t, true)

	// The unsupported .tiff and the SVG-wrapped image; the cover is stored as
	// the cover and the plate was kept, so neither counts.
	if book.DroppedImages != 2 {
		t.Fatalf("DroppedImages = %d, want 2", book.DroppedImages)
	}

	for _, chapter := range book.Chapters {
		if strings.Contains(chapter.Text, ".tiff") || strings.Contains(chapter.Text, ".svg") {
			t.Errorf("text references artwork that was not stored:\n%s", chapter.Text)
		}
	}
}

func TestKeepImagesDisabledLeavesTheOldBehaviour(t *testing.T) {
	book := parseIllustrated(t, false)

	if len(book.Images) != 0 {
		t.Fatalf("stored %d images with KeepImages off, want 0", len(book.Images))
	}
	// Every distinct illustration is a loss again: plate, tiff and svg.
	if book.DroppedImages != 3 {
		t.Fatalf("DroppedImages = %d, want 3", book.DroppedImages)
	}
	for _, chapter := range book.Chapters {
		if strings.Contains(chapter.Text, "![") {
			t.Errorf("text carries an image link with KeepImages off:\n%s", chapter.Text)
		}
	}
}

// A plain-text book has no image syntax, so the strategy must not ask for
// illustrations even when the setting says to keep them.
func TestPlainPresetNeverKeepsImages(t *testing.T) {
	keep := true
	plain := Strategy{Preset: PresetPlain, KeepImages: &keep}
	if plain.ParseOptions().KeepImages {
		t.Fatal("plain preset asked to keep images")
	}

	markdown := Strategy{Preset: PresetMarkdown, KeepImages: &keep}
	if !markdown.ParseOptions().KeepImages {
		t.Fatal("markdown preset did not ask to keep images")
	}
}

// A strategy stored before keep_images existed must keep illustrations rather
// than inherit a false zero value.
func TestUnspecifiedKeepImagesDefaultsToOn(t *testing.T) {
	if !(Strategy{Preset: PresetMarkdown}).ParseOptions().KeepImages {
		t.Fatal("an unspecified keep_images did not default to on")
	}

	off := false
	if (Strategy{Preset: PresetMarkdown, KeepImages: &off}).ParseOptions().KeepImages {
		t.Fatal("an explicit keep_images=false was ignored")
	}
}

func TestKeepImagesRespectsThePerImageBudget(t *testing.T) {
	entries := illustratedEntries()
	for i := range entries {
		if entries[i].name == "OEBPS/images/plate.png" {
			entries[i].body = strings.Repeat("x", maxImageBytes+1)
		}
	}

	book, err := parseArchive(t, buildArchive(t, entries), Options{MarkdownInline: true, KeepImages: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(book.Images) != 0 {
		t.Fatalf("stored %d oversized images, want 0", len(book.Images))
	}
	// An oversized image is a loss like any other, and the text must not point
	// at a file that was never written.
	if book.DroppedImages != 3 {
		t.Fatalf("DroppedImages = %d, want 3", book.DroppedImages)
	}
	for _, chapter := range book.Chapters {
		if strings.Contains(chapter.Text, "![") {
			t.Errorf("text links an image that exceeded the budget:\n%s", chapter.Text)
		}
	}
}

func TestSanitizeImageAlt(t *testing.T) {
	// Brackets would leave the reader unable to parse the line, and a newline
	// would break the one-image-per-line shape it requires.
	if got := sanitizeImageAlt("a [bracketed] alt\nsecond line"); got != "a bracketed alt second line" {
		t.Fatalf("sanitizeImageAlt = %q", got)
	}
}
