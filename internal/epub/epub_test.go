package epub

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	body string
}

func buildArchive(t *testing.T, entries []zipEntry) []byte {
	t.Helper()

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

func parseArchive(t *testing.T, data []byte, opts Options) (*Book, error) {
	t.Helper()
	return Parse(bytes.NewReader(data), int64(len(data)), opts)
}

const containerXML = `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

// epub3Entries is a minimal but realistic EPUB 3: a navigation document, two
// chapters, a cover image, and metadata carrying a scheme-less urn identifier.
func epub3Entries() []zipEntry {
	return []zipEntry{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>星塵集</dc:title>
    <dc:creator>林望舒</dc:creator>
    <dc:language>zh-Hant</dc:language>
    <dc:description>一部關於旅途的短篇小說。</dc:description>
    <dc:date>2024-03-01</dc:date>
    <dc:identifier id="pub-id">urn:isbn:9781234567897</dc:identifier>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="cover-img" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="nav"/>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
  </spine>
</package>`},
		{"OEBPS/nav.xhtml", `<html xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
  <li><a href="text/ch1.xhtml">啟程</a></li>
  <li><a href="text/ch2.xhtml">歸途</a></li>
</ol></nav></body></html>`},
		{"OEBPS/images/cover.png", "\x89PNG-not-a-real-image"},
		{"OEBPS/text/ch1.xhtml", `<html><body>
  <h1>第一章</h1>
  <p>他走出了車站，
     天色still早。</p>
  <p>風很大。</p>
</body></html>`},
		{"OEBPS/text/ch2.xhtml", `<html><body>
  <h2>第二章</h2>
  <p>回程的路上，他<em>沉默</em>不語。</p>
</body></html>`},
	}
}

func TestParseEPUB3(t *testing.T) {
	book, err := parseArchive(t, buildArchive(t, epub3Entries()), Options{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if book.Title != "星塵集" {
		t.Errorf("Title = %q, want %q", book.Title, "星塵集")
	}
	if len(book.Authors) != 1 || book.Authors[0] != "林望舒" {
		t.Errorf("Authors = %v, want [林望舒]", book.Authors)
	}
	if book.Language != "zh-Hant" {
		t.Errorf("Language = %q, want zh-Hant", book.Language)
	}
	if book.Description != "一部關於旅途的短篇小說。" {
		t.Errorf("Description = %q", book.Description)
	}
	if book.PublishedAt != "2024-03-01" {
		t.Errorf("PublishedAt = %q, want 2024-03-01", book.PublishedAt)
	}
	if got := book.Identifiers["isbn"]; got != "urn:isbn:9781234567897" {
		t.Errorf("Identifiers[isbn] = %q", got)
	}
	if string(book.Cover) != "\x89PNG-not-a-real-image" {
		t.Errorf("Cover = %q, want the manifest cover-image entry", string(book.Cover))
	}
	if book.CoverExt != ".png" {
		t.Errorf("CoverExt = %q, want .png", book.CoverExt)
	}

	// The navigation document is in the spine but is not a chapter.
	if len(book.Chapters) != 2 {
		t.Fatalf("len(Chapters) = %d, want 2; chapters = %+v", len(book.Chapters), book.Chapters)
	}

	// Titles come from the navigation document, which wins over the in-document
	// heading.
	if book.Chapters[0].Title != "啟程" {
		t.Errorf("Chapters[0].Title = %q, want 啟程", book.Chapters[0].Title)
	}
	if book.Chapters[1].Title != "歸途" {
		t.Errorf("Chapters[1].Title = %q, want 歸途", book.Chapters[1].Title)
	}

	// The heading consumed as the fallback title must not also appear in the
	// body, the source line wrap inside the paragraph must not leave a space
	// between the CJK characters, and paragraphs are separated by a blank line.
	wantText := "他走出了車站，天色still早。\n\n風很大。"
	if got := book.Chapters[0].Text; got != wantText {
		t.Errorf("Chapters[0].Text = %q, want %q", got, wantText)
	}

	// The cover is kept, and no chapter references an illustration.
	if book.DroppedImages != 0 {
		t.Errorf("DroppedImages = %d, want 0", book.DroppedImages)
	}
}

func TestParseDroppedImages(t *testing.T) {
	// coverOPF declares a cover image plus the documents each case needs, so the
	// interesting difference between cases stays in the chapter markup.
	const coverOPF = `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>插圖</dc:title></metadata>
  <manifest>
    <item id="cover-img" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c0" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c0"/><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`

	tests := []struct {
		name    string
		entries []zipEntry
		want    int
	}{
		{
			name: "chapter illustrations are counted",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/images/plate1.png", "plate-bytes"},
				{"OEBPS/images/plate2.png", "plate-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><p>書名</p></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p><img src="../images/plate1.png"/></body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><p>內文。</p><img src="../images/plate2.png"/></body></html>`},
			},
			want: 2,
		},
		{
			// Without this exclusion nearly every EPUB would report a loss: the
			// cover page normally sits in the spine and points at the cover.
			name: "cover page image is not counted",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><img src="../images/cover.png"/></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 0,
		},
		{
			// The flattener skips <svg> as a whole subtree, so the counter has to
			// descend into it separately. Expecting a non-zero count here is what
			// makes the cover case below meaningful: a counter that never entered
			// an <svg> at all would satisfy that one by accident.
			name: "svg wrapped illustration is counted",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/images/plate1.png", "plate-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><p>書名</p></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p><svg xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="../images/plate1.png"/></svg></body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 1,
		},
		{
			// The other common cover-page shape.
			name: "svg wrapped cover image is not counted",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><svg xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 100 100">
  <image width="100" height="100" xlink:href="../images/cover.png"/>
</svg></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 0,
		},
		{
			name: "an illustration repeated across chapters counts once",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/images/ornament.png", "ornament-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><p>書名</p></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p><img src="../images/ornament.png"/></body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><img src="../images/ornament.png"/><p>內文。</p></body></html>`},
			},
			want: 1,
		},
		{
			// No cover is stored here, so the frontispiece really is lost — even
			// though its document flattens to nothing and is skipped as a chapter.
			name: "image on a skipped page counts when no cover is stored",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>無封面</dc:title></metadata>
  <manifest>
    <item id="c0" href="text/front.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c0"/><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/images/frontispiece.png", "frontispiece-bytes"},
				{"OEBPS/text/front.xhtml", `<html><body><img src="../images/frontispiece.png"/></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 1,
		},
		{
			// The archive stores the cover one way and the cover page spells it
			// another. lookupZipEntry reads both, so the exclusion has to hold too
			// or the book reports a loss it did not suffer.
			name: "cover referenced with different casing is still excluded",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>大小寫</dc:title></metadata>
  <manifest>
    <item id="cover-img" href="Images/Cover.PNG" media-type="image/png" properties="cover-image"/>
    <item id="c0" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c0"/><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/Images/Cover.PNG", "cover-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><img src="../images/cover.png"/></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 0,
		},
		{
			// Nothing was lost: the archive never carried these. Counting a href
			// the file cannot satisfy would report artwork that never existed.
			name: "references the archive cannot satisfy are not counted",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><p>書名</p></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p>
  <img src="../images/missing.png"/>
  <img src="https://example.com/remote.png"/>
  <img src="data:image/png;base64,iVBORw0KGgo="/>
</body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 0,
		},
		{
			name: "an image with no reference is ignored",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", coverOPF},
				{"OEBPS/images/cover.png", "cover-bytes"},
				{"OEBPS/text/cover.xhtml", `<html><body><p>書名</p></body></html>`},
				{"OEBPS/text/ch1.xhtml", `<html><body><p>內文。</p><img alt="broken"/></body></html>`},
				{"OEBPS/text/ch2.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book, err := parseArchive(t, buildArchive(t, tt.entries), Options{})
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if book.DroppedImages != tt.want {
				t.Errorf("DroppedImages = %d, want %d", book.DroppedImages, tt.want)
			}
		})
	}
}

func TestParseEPUB2WithNCX(t *testing.T) {
	entries := []zipEntry{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>舊城手記</dc:title>
    <dc:creator>佚名</dc:creator>
    <dc:identifier id="bookid" opf:scheme="ISBN">9789999999999</dc:identifier>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="cover-img" href="cover.jpeg" media-type="image/jpeg"/>
    <item id="c1" href="ch1.html" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="c1"/>
  </spine>
</package>`},
		{"OEBPS/toc.ncx", `<?xml version="1.0"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint id="n1" playOrder="1">
      <navLabel><text>序章</text></navLabel>
      <content src="ch1.html"/>
      <navPoint id="n1a" playOrder="2">
        <navLabel><text>巷口</text></navLabel>
        <content src="ch1.html#sec"/>
      </navPoint>
    </navPoint>
  </navMap>
</ncx>`},
		{"OEBPS/cover.jpeg", "jpeg-bytes"},
		{"OEBPS/ch1.html", `<html><body><p>城牆下的雨。</p></body></html>`},
	}

	book, err := parseArchive(t, buildArchive(t, entries), Options{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(book.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1", len(book.Chapters))
	}
	if book.Chapters[0].Title != "序章" {
		t.Errorf("Chapters[0].Title = %q, want 序章 (outer navPoint wins)", book.Chapters[0].Title)
	}
	if got := book.Identifiers["isbn"]; got != "9789999999999" {
		t.Errorf("Identifiers[isbn] = %q, want the opf:scheme value", got)
	}
	if string(book.Cover) != "jpeg-bytes" {
		t.Errorf("Cover = %q, want the meta name=cover target", string(book.Cover))
	}
	if book.CoverExt != ".jpeg" {
		t.Errorf("CoverExt = %q, want .jpeg", book.CoverExt)
	}
}

func TestParseFallbacks(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipEntry
		check   func(t *testing.T, book *Book)
	}{
		{
			name: "no table of contents falls back to document heading",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>無目錄</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/ch1.xhtml", `<html><body><h3>第十七章 夜行</h3><p>內文。</p></body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if book.Chapters[0].Title != "第十七章 夜行" {
					t.Errorf("Title = %q, want the h3 text", book.Chapters[0].Title)
				}
				if book.Chapters[0].Text != "內文。" {
					t.Errorf("Text = %q, want just the paragraph", book.Chapters[0].Text)
				}
			},
		},
		{
			name: "no cover leaves Cover nil",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>無封面</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if book.Cover != nil {
					t.Errorf("Cover = %q, want nil", string(book.Cover))
				}
				if book.CoverExt != "" {
					t.Errorf("CoverExt = %q, want empty", book.CoverExt)
				}
			},
		},
		{
			name: "undeclared html entities do not fail the parse",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>壞實體</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
				// &nbsp; is undeclared in XHTML 1.1 without a DTD and would break a
				// strict XML decoder. An unclosed <p> would too.
				{"OEBPS/ch1.xhtml", `<html><body><p>前面&nbsp;後面<p>第二段</body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if len(book.Chapters) != 1 {
					t.Fatalf("len(Chapters) = %d, want 1", len(book.Chapters))
				}
				if !strings.Contains(book.Chapters[0].Text, "前面 後面") {
					t.Errorf("Text = %q, want the nbsp folded to a space", book.Chapters[0].Text)
				}
				if !strings.Contains(book.Chapters[0].Text, "第二段") {
					t.Errorf("Text = %q, want the unclosed second paragraph", book.Chapters[0].Text)
				}
			},
		},
		{
			name: "missing container falls back to any opf entry",
			entries: []zipEntry{
				{"book.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>無容器</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
				{"ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if book.Title != "無容器" {
					t.Errorf("Title = %q, want 無容器", book.Title)
				}
			},
		},
		{
			name: "percent-encoded and fragmented hrefs resolve",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>編碼</dc:title></metadata>
  <manifest><item id="c1" href="text/%E7%AC%AC%E4%B8%80%E7%AB%A0.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/text/第一章.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if len(book.Chapters) != 1 {
					t.Fatalf("len(Chapters) = %d, want 1", len(book.Chapters))
				}
			},
		},
		{
			name: "spine entry missing from manifest is skipped",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>缺項</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="ghost"/><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/ch1.xhtml", `<html><body><p>內文。</p></body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if len(book.Chapters) != 1 {
					t.Fatalf("len(Chapters) = %d, want 1", len(book.Chapters))
				}
			},
		},
		{
			name: "ruby annotations are dropped",
			entries: []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>ルビ</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
				{"OEBPS/ch1.xhtml", `<html><body><p><ruby>漢字<rp>(</rp><rt>かんじ</rt><rp>)</rp></ruby>を読む</p></body></html>`},
			},
			check: func(t *testing.T, book *Book) {
				if got := book.Chapters[0].Text; got != "漢字を読む" {
					t.Errorf("Text = %q, want the base text without furigana", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book, err := parseArchive(t, buildArchive(t, tt.entries), Options{})
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			tt.check(t, book)
		})
	}
}

func TestParseMarkdownInline(t *testing.T) {
	entries := []zipEntry{
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>強調</dc:title></metadata>
  <manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
		{"OEBPS/ch1.xhtml", `<html><body><p>He said <em> softly </em>and <strong>loudly</strong>.</p></body></html>`},
	}
	data := buildArchive(t, entries)

	t.Run("plain drops markers", func(t *testing.T) {
		book, err := parseArchive(t, data, Options{})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got := book.Chapters[0].Text; got != "He said softly and loudly." {
			t.Errorf("Text = %q, want unmarked text", got)
		}
	})

	t.Run("markdown keeps markers outside padding", func(t *testing.T) {
		book, err := parseArchive(t, data, Options{MarkdownInline: true})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got := book.Chapters[0].Text; got != "He said *softly* and **loudly**." {
			t.Errorf("Text = %q, want emphasis markers hugging the words", got)
		}
	})
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{
			name:    "not a zip",
			data:    []byte("this is a plain text file, not an epub"),
			wantErr: "not a readable epub archive",
		},
		{
			name: "no package document",
			data: buildArchive(t, []zipEntry{
				{"mimetype", "application/epub+zip"},
				{"random.txt", "hello"},
			}),
			wantErr: "no package document",
		},
		{
			name: "no readable chapters",
			data: buildArchive(t, []zipEntry{
				{"META-INF/container.xml", containerXML},
				{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>空</dc:title></metadata>
  <manifest/>
  <spine/>
</package>`},
			}),
			wantErr: "no readable chapters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArchive(t, tt.data, Options{})
			if err == nil {
				t.Fatalf("Parse() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Parse() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseRejectsOversizedSpineDocument(t *testing.T) {
	entries := []zipEntry{
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>過大章節</dc:title></metadata>
  <manifest>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`},
		{"OEBPS/ch1.xhtml", `<html><body><p>可讀章節。</p></body></html>`},
		{"OEBPS/ch2.xhtml", strings.Repeat("x", maxDocumentBytes+1)},
	}

	_, err := parseArchive(t, buildArchive(t, entries), Options{})
	if err == nil {
		t.Fatal("Parse() error = nil, want the oversized spine document to reject the import")
	}
	if !errors.Is(err, errZipEntryTooLarge) {
		t.Fatalf("Parse() error = %v, want errZipEntryTooLarge", err)
	}
}

func TestResolveHref(t *testing.T) {
	tests := []struct {
		baseDir string
		href    string
		want    string
	}{
		{"OEBPS", "text/ch1.xhtml", "OEBPS/text/ch1.xhtml"},
		{"OEBPS", "text/ch1.xhtml#top", "OEBPS/text/ch1.xhtml"},
		{"OEBPS/text", "../images/cover.png", "OEBPS/images/cover.png"},
		{"", "ch1.xhtml", "ch1.xhtml"},
		{"OEBPS", "/absolute/ch1.xhtml", "absolute/ch1.xhtml"},
		{"OEBPS", "%E7%AC%AC.xhtml", "OEBPS/第.xhtml"},
		{"OEBPS", "#only-a-fragment", ""},
		{"OEBPS", "  ", ""},
	}

	for _, tt := range tests {
		if got := resolveHref(tt.baseDir, tt.href); got != tt.want {
			t.Errorf("resolveHref(%q, %q) = %q, want %q", tt.baseDir, tt.href, got, tt.want)
		}
	}
}
