package testutil

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"testing"
)

const ComicEPUBTitle = "Comic Chapter 01"

func PNGOfWidth(t *testing.T, width int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, width, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func BuildComicTestEPUB(t *testing.T, pages int) []byte {
	t.Helper()
	manifest, spine := "", ""
	entries := []struct{ name, body string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`},
		{"cover.xhtml", `<html><head><title>Cover</title></head><body><div><img src="cover.jpg" alt="cover"/></div></body></html>`},
		{"cover.jpg", string(PNGOfWidth(t, 100))},
		{"1/titlepage.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><head><meta name="calibre:cover" content="true"/><title>Cover</title></head>
<body><div><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 1 1"><image width="1" height="1" xlink:href="cover.jpeg"/></svg></div></body></html>`},
		{"1/cover.jpeg", string(PNGOfWidth(t, 101))},
	}
	for n := 1; n <= pages; n++ {
		manifest += fmt.Sprintf(`<item id="p%d" href="1/page_%d.xhtml" media-type="application/xhtml+xml"/>`, n, n)
		spine += fmt.Sprintf(`<itemref idref="p%d"/>`, n)
		entries = append(entries,
			struct{ name, body string }{fmt.Sprintf("1/page_%d.xhtml", n), fmt.Sprintf(`<html><head><title>Page #%d</title></head><body class="calibre"><div><img src="%d_0.png" alt="comic page #%d"/></div></body></html>`, n, n-1, n)},
			struct{ name, body string }{fmt.Sprintf("1/%d_0.png", n-1), string(PNGOfWidth(t, n))},
		)
	}
	entries = append(entries, struct{ name, body string }{"content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:isbn:9781234567897</dc:identifier>
    <dc:title>` + ComicEPUBTitle + `</dc:title>
    <dc:creator>Comic Author</dc:creator>
    <dc:language>zh-Hant</dc:language>
    <dc:description>Comic summary</dc:description>
    <meta name="cover" content="coverimage"/>
  </metadata>
  <manifest>
    <item id="coverimage" href="cover.jpg" media-type="image/jpeg"/>
    <item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="title" href="1/titlepage.xhtml" media-type="application/xhtml+xml"/>
    ` + manifest + `
  </manifest>
  <spine><itemref idref="cover"/><itemref idref="title"/>` + spine + `</spine>
  <guide><reference type="cover" title="Cover" href="cover.xhtml"/></guide>
</package>`})
	return ZipEPUBEntries(t, entries)
}
