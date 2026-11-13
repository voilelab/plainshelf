package epub

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"path"
	"strings"

	"golang.org/x/net/html"

	"github.com/voilelab/plainshelf/internal/cbz"
	"github.com/voilelab/plainshelf/internal/util"
)

const (
	maxComicPages      = 2000
	maxComicImageBytes = 2 << 30
)

type Comic struct {
	Title       string
	Authors     []string
	Language    string
	Description string
	Identifiers map[string]string
	PublishedAt string
	Direction   string
	Pages       []cbz.Page
}

type comicDocument struct {
	cover  bool
	images []*zip.File
}

func ParseComic(r io.ReaderAt, size int64) (*Comic, error) {
	opened, err := openPackage(r, size)
	if err != nil {
		return nil, err
	}
	pkg := opened.pkg

	docs, err := readComicDocuments(opened)
	if err != nil {
		return nil, err
	}

	entries := comicPageEntries(docs, false)
	if len(entries) == 0 {
		entries = comicPageEntries(docs, true)
	}
	if len(entries) == 0 {
		return nil, util.NewError("epub contains no page images")
	}
	if len(entries) > maxComicPages {
		return nil, util.Errorf("epub exceeds %d page limit", maxComicPages)
	}

	comic := &Comic{
		Title:       firstNonEmpty(pkg.Metadata.Titles),
		Authors:     nonEmpty(pkg.Metadata.Creators),
		Language:    firstNonEmpty(pkg.Metadata.Languages),
		Description: firstNonEmpty(pkg.Metadata.Descriptions),
		PublishedAt: firstNonEmpty(pkg.Metadata.Dates),
		Identifiers: collectIdentifiers(pkg.Metadata.Identifiers),
	}
	if strings.EqualFold(strings.TrimSpace(pkg.Spine.PageProgressionDirection), "rtl") {
		comic.Direction = "rtl"
	}

	var total uint64
	for _, entry := range entries {
		ext := strings.ToLower(path.Ext(entry.Name))
		if !imageExtensions[ext] {
			return nil, util.Errorf("epub page image %q is not a .jpg, .jpeg, .png, .webp or .gif file", entry.Name)
		}
		total += entry.UncompressedSize64
		if total > maxComicImageBytes {
			return nil, util.NewError("epub page images exceed 2 GiB total uncompressed size limit")
		}
		comic.Pages = append(comic.Pages, cbz.NewPage(entry))
	}

	return comic, nil
}

func readComicDocuments(opened *epubPackage) ([]comicDocument, error) {
	coverDocs := make(map[string]bool)
	for _, ref := range opened.pkg.Guide.References {
		if strings.EqualFold(strings.TrimSpace(ref.Type), "cover") {
			coverDocs[resolveHref(opened.baseDir, ref.Href)] = true
		}
	}

	var docs []comicDocument
	var total int64
	for _, ref := range opened.pkg.Spine.ItemRefs {
		item, ok := opened.manifest[ref.IDRef]
		if !ok || !isDocumentMediaType(item.MediaType) || hasProperty(item.Properties, "nav") {
			continue
		}

		docPath := resolveHref(opened.baseDir, item.Href)
		data, err := readZipEntryLimit(opened.files, docPath, maxDocumentBytes)
		if err != nil {
			if errors.Is(err, errZipEntryTooLarge) {
				return nil, util.Errorf("failed to read spine document %q: %w", docPath, err)
			}
			continue
		}
		total += int64(len(data))
		if total > maxTotalBytes {
			return nil, util.NewError("epub content exceeds the maximum supported size")
		}

		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, util.Errorf("failed to parse spine document %q: %w", docPath, err)
		}
		body := findElement(doc, "body")
		if body == nil {
			body = doc
		}
		if hasVisibleText(body) {
			return nil, util.Errorf("epub page %q contains text; import it as Markdown or plain text instead", docPath)
		}

		docDir := path.Dir(docPath)
		var images []*zip.File
		for _, href := range collectImageHrefs(body) {
			entry, ok := lookupZipEntry(opened.files, resolveHref(docDir, href))
			if !ok {
				return nil, util.Errorf("epub page %q references missing image %q", docPath, href)
			}
			images = append(images, entry)
		}

		docs = append(docs, comicDocument{
			cover:  coverDocs[docPath] || isCalibreCoverPage(doc),
			images: images,
		})
	}

	return docs, nil
}

func comicPageEntries(docs []comicDocument, includeCovers bool) []*zip.File {
	seen := make(map[*zip.File]bool)
	var entries []*zip.File
	for _, doc := range docs {
		if doc.cover && !includeCovers {
			continue
		}
		for _, entry := range doc.images {
			if seen[entry] {
				continue
			}
			seen[entry] = true
			entries = append(entries, entry)
		}
	}
	return entries
}

func hasVisibleText(n *html.Node) bool {
	switch n.Type {
	case html.TextNode:
		return strings.TrimSpace(n.Data) != ""
	case html.ElementNode:
		if _, skip := skipTags[n.Data]; skip {
			return false
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasVisibleText(c) {
			return true
		}
	}
	return false
}

func isCalibreCoverPage(doc *html.Node) bool {
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" &&
			strings.EqualFold(attr(n, "name"), "calibre:cover") &&
			strings.EqualFold(strings.TrimSpace(attr(n, "content")), "true") {
			found = true
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}
