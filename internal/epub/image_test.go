package epub

import (
	"strings"
	"testing"
)

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

func TestKeepImagesStillCountsWhatItCannotStore(t *testing.T) {
	book := parseIllustrated(t, true)

	if book.DroppedImages != 2 {
		t.Fatalf("DroppedImages = %d, want 2", book.DroppedImages)
	}

	for _, chapter := range book.Chapters {
		if strings.Contains(chapter.Text, ".tiff") || strings.Contains(chapter.Text, ".svg") {
			t.Errorf("text references artwork that was not stored:\n%s", chapter.Text)
		}
	}
}

func TestKeepImagesDisabledCountsEveryIllustrationAsDropped(t *testing.T) {
	book := parseIllustrated(t, false)

	if len(book.Images) != 0 {
		t.Fatalf("stored %d images with KeepImages off, want 0", len(book.Images))
	}
	if book.DroppedImages != 3 {
		t.Fatalf("DroppedImages = %d, want 3", book.DroppedImages)
	}
	for _, chapter := range book.Chapters {
		if strings.Contains(chapter.Text, "![") {
			t.Errorf("text carries an image link with KeepImages off:\n%s", chapter.Text)
		}
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
	if got := sanitizeImageAlt("a [bracketed] alt\nsecond line"); got != "a bracketed alt second line" {
		t.Fatalf("sanitizeImageAlt = %q", got)
	}
}

func TestIllustrationInsideConsumedHeadingCountsAsDropped(t *testing.T) {
	entries := []zipEntry{
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <manifest><item id="c1" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
		{"OEBPS/chapter.xhtml", `<html><body><h1>Chapter<img src="heading.png"/></h1><p>Body.</p></body></html>`},
		{"OEBPS/heading.png", "heading-artwork"},
	}
	book, err := parseArchive(t, buildArchive(t, entries), Options{MarkdownInline: true, KeepImages: true})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(book.Chapters) != 1 || book.Chapters[0].Title != "Chapter" || book.Chapters[0].Text != "Body." {
		t.Fatalf("Chapters = %+v, want the heading consumed and the body preserved", book.Chapters)
	}
	if len(book.Images) != 0 || book.DroppedImages != 1 {
		t.Errorf("kept %d images, dropped %d, want the consumed heading's image counted once as dropped", len(book.Images), book.DroppedImages)
	}
}
