package testutil

import (
	"archive/zip"
	"bytes"
	"testing"
)

// TestEPUBTitle is the dc:title inside the archive buildTestEPUB produces.
const TestEPUBTitle = "星塵集"

// buildTestEPUB returns a small but complete EPUB 3 archive: two chapters, a
// navigation document, a cover, and enough metadata to exercise every field the
// importer copies onto the book.
func BuildTestEPUB(t *testing.T) []byte {
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
    <dc:title>` + TestEPUBTitle + `</dc:title>
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
		{"OEBPS/cover.png", string(OnePixelPNG())},
		{"OEBPS/ch1.xhtml", `<html><body><h1>第一章</h1><p>他走出了車站。</p></body></html>`},
		{"OEBPS/ch2.xhtml", `<html><body><h1>第二章</h1><p>回程的路上。</p></body></html>`},
	}

	return ZipEPUBEntries(t, entries)
}

// BuildIllustratedTestEPUB is BuildTestEPUB with two illustrations in the
// chapters, for the paths that report what conversion discarded.
func BuildIllustratedTestEPUB(t *testing.T) []byte {
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
    <dc:title>` + TestEPUBTitle + `</dc:title>
    <dc:language>zh-Hant</dc:language>
    <dc:identifier id="pub-id">urn:isbn:9781234567897</dc:identifier>
  </metadata>
  <manifest>
    <item id="cover-img" href="cover.png" media-type="image/png" properties="cover-image"/>
    <item id="plate1" href="images/plate1.png" media-type="image/png"/>
    <item id="plate2" href="images/plate2.png" media-type="image/png"/>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`},
		{"OEBPS/cover.png", string(OnePixelPNG())},
		{"OEBPS/images/plate1.png", string(OnePixelPNG())},
		{"OEBPS/images/plate2.png", string(OnePixelPNG())},
		{"OEBPS/ch1.xhtml", `<html><body><h1>第一章</h1><p>他走出了車站。</p><img src="images/plate1.png"/></body></html>`},
		{"OEBPS/ch2.xhtml", `<html><body><h1>第二章</h1><p>回程的路上。</p><img src="images/plate2.png"/></body></html>`},
	}

	return ZipEPUBEntries(t, entries)
}

// BuildUnstorableImageTestEPUB carries one illustration in a format the shelf
// does not serve, which is what keeps the dropped-images note meaningful now
// that ordinary artwork survives the import.
func BuildUnstorableImageTestEPUB(t *testing.T) []byte {
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
    <dc:title>` + TestEPUBTitle + `</dc:title>
    <dc:language>zh-Hant</dc:language>
    <dc:identifier id="pub-id">urn:isbn:9781234567897</dc:identifier>
  </metadata>
  <manifest>
    <item id="scan" href="images/scan.tiff" media-type="image/tiff"/>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>`},
		{"OEBPS/images/scan.tiff", "tiff-bytes"},
		{"OEBPS/ch1.xhtml", `<html><body><h1>第一章</h1><p>他走出了車站。</p><img src="images/scan.tiff"/></body></html>`},
	}

	return ZipEPUBEntries(t, entries)
}

func ZipEPUBEntries(t *testing.T, entries []struct{ name, body string }) []byte {
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

// OnePixelPNG is a real 1x1 PNG so the cover survives the JPEG conversion the
// import performs when cover_to_jpg is enabled.
func OnePixelPNG() []byte {
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
