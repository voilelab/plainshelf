// Package epub extracts book metadata and chapter text from EPUB files.
//
// The package is deliberately import-only: it converts an EPUB into the plain
// data PlainShelf already stores (a title, some metadata, a cover image, and an
// ordered list of chapters). It never renders EPUB, and it does not preserve the
// original archive. Embedded illustrations are kept only when Options.KeepImages
// asks for them, and are reported as dropped otherwise.
package epub

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// Size guards for untrusted archives. A malicious or simply broken EPUB can
// declare a small compressed size and expand to something unbounded, so both a
// per-document and a whole-book ceiling are enforced while reading.
const (
	maxDocumentBytes = 32 << 20  // 32 MB for a single spine document
	maxTotalBytes    = 256 << 20 // 256 MB across all spine documents

	// Illustrations get a tighter budget: every kept image is held in memory
	// until the book is written and then lives on the shelf forever. One past
	// either ceiling is dropped and counted, so an outsized archive costs its
	// pictures rather than the whole import.
	maxImageBytes      = 8 << 20  // 8 MB for a single illustration
	maxTotalImageBytes = 64 << 20 // 64 MB across all illustrations in one book
)

var errZipEntryTooLarge = errors.New("zip entry exceeds the maximum supported size")

// ImageFolder is written into the Markdown targets this package produces. It is
// declared here rather than imported so an EPUB parser does not depend on the
// shelf's storage layer; server/import_epub_test.go asserts it still agrees with
// shelf.SourceAssetsFolder, where the files actually land.
const ImageFolder = "assets"

// imageExtensions are the formats worth keeping; anything else, SVG included, is
// dropped and counted. The same drift test covers this against
// shelf.IsSupportedImageExt: an image the shelf refuses would be referenced by
// the text and never load.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// Chapter is one spine document, flattened to text.
type Chapter struct {
	// Title comes from the table of contents when the document is listed there,
	// otherwise from the document's own first heading. It may be empty.
	Title string

	// Text is the document body with markup removed. Paragraphs are separated by
	// a blank line. When Options.MarkdownInline is set, emphasis is preserved as
	// Markdown markers; otherwise the text carries no markup at all.
	Text string
}

// Image is one illustration kept from an EPUB, ready to be stored beside the
// text that references it.
type Image struct {
	// Name is the file name the text refers to, always a bare name with a
	// supported image extension so the shelf accepts it as an asset.
	Name string
	Data []byte
}

// Book is everything import needs from an EPUB.
type Book struct {
	Title       string
	Authors     []string
	Language    string
	Description string

	// Identifiers holds dc:identifier values keyed by their scheme (lower-cased,
	// e.g. "isbn"). Values with no usable scheme are keyed "identifier".
	Identifiers map[string]string

	// PublishedAt is the raw dc:date string. EPUB does not constrain the format
	// beyond a recommendation, so parsing is left to the caller.
	PublishedAt string

	// Cover is the cover image bytes, or nil when the EPUB declares none. The
	// format is whatever the EPUB stored; callers that need JPEG should run it
	// through internal/imgutil.
	Cover    []byte
	CoverExt string

	// Images holds the illustrations kept from the spine, in the order they
	// were first referenced. It is empty unless Options.KeepImages was set.
	Images []Image

	// DroppedImages counts the distinct illustrations referenced by the spine
	// documents that the conversion discarded, so the number reflects artwork
	// lost rather than tags removed. An asset no spine document references is not
	// counted, and neither is the stored cover, which is kept rather than lost.
	DroppedImages int

	Chapters []Chapter
}

// Options controls how chapter text is produced.
type Options struct {
	// MarkdownInline keeps <em>/<strong> as Markdown emphasis markers. Leave it
	// false when the output is stored as plain text, or the markers show up
	// literally in the reader.
	MarkdownInline bool

	// KeepImages stores the illustrations the spine references and writes a
	// Markdown image link where each one appeared. It only makes sense
	// alongside MarkdownInline: in plain text the link would show up literally.
	KeepImages bool

	// imageTarget maps a raw href in the document currently being flattened to
	// the Markdown target to write for it, or "" to drop it as before.
	//
	// It is unexported and set per document by Parse, which is the only place
	// that knows the archive, the document's own directory, and the names the
	// images are being stored under.
	imageTarget func(href string) string
}

// Parse reads an EPUB from r and returns its metadata and chapter text.
//
// Parsing is tolerant on purpose: a missing table of contents, a missing cover,
// or a spine entry that is not in the manifest degrade gracefully rather than
// failing the import. Only a structurally unusable archive (not a zip, no
// container.xml, no readable package document, no chapters) returns an error.
func Parse(r io.ReaderAt, size int64, opts Options) (*Book, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, util.Errorf("not a readable epub archive: %w", err)
	}

	files := indexZip(zr)

	opfPath, err := findPackagePath(files)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	opfData, err := readZipEntry(files, opfPath)
	if err != nil {
		return nil, util.Errorf("failed to read package document %q: %w", opfPath, err)
	}

	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return nil, util.Errorf("failed to parse package document %q: %w", opfPath, err)
	}

	baseDir := path.Dir(opfPath)
	if baseDir == "." {
		baseDir = ""
	}

	manifest := make(map[string]opfItem, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		manifest[item.ID] = item
	}

	book := &Book{
		Title:       firstNonEmpty(pkg.Metadata.Titles),
		Authors:     nonEmpty(pkg.Metadata.Creators),
		Language:    firstNonEmpty(pkg.Metadata.Languages),
		Description: firstNonEmpty(pkg.Metadata.Descriptions),
		PublishedAt: firstNonEmpty(pkg.Metadata.Dates),
		Identifiers: collectIdentifiers(pkg.Metadata.Identifiers),
	}

	var coverPath string
	book.Cover, book.CoverExt, coverPath = readCover(files, manifest, pkg, baseDir)

	titles := readTOCTitles(files, manifest, pkg, baseDir)

	// Illustrations are collected across the whole spine and deduplicated by
	// archive entry, so an ornament repeated on every chapter counts once. The
	// zip entry rather than the resolved path is the identity: lookupZipEntry
	// folds case, so two documents spelling the same file differently still
	// land on one image.
	droppedImages := make(map[*zip.File]struct{})

	// keptImages maps an archive entry to the name it was stored under, keyed
	// the same way as droppedImages so an illustration can never be both.
	keptImages := make(map[*zip.File]string)
	var imageBytes int64

	// keepImage resolves one reference and reports the Markdown target to write
	// for it, or "" when the illustration is not kept. Every "" path also marks
	// the entry dropped, so the note the import leaves stays truthful.
	keepImage := func(docDir, href string) string {
		entry, ok := lookupZipEntry(files, resolveHref(docDir, href))
		if !ok {
			// A stale link, an external URL or a data: URI names artwork this
			// file never carried; nothing was lost, so nothing is counted.
			return ""
		}
		if name, ok := keptImages[entry]; ok {
			return path.Join(ImageFolder, name)
		}
		if _, dropped := droppedImages[entry]; dropped {
			return ""
		}

		ext := strings.ToLower(path.Ext(entry.Name))
		if !imageExtensions[ext] {
			droppedImages[entry] = struct{}{}
			return ""
		}

		data, err := readZipEntryLimit(files, entry.Name, maxImageBytes)
		if err != nil || imageBytes+int64(len(data)) > maxTotalImageBytes {
			droppedImages[entry] = struct{}{}
			return ""
		}
		imageBytes += int64(len(data))

		name := fmt.Sprintf("img-%04d%s", len(book.Images)+1, ext)
		book.Images = append(book.Images, Image{Name: name, Data: data})
		keptImages[entry] = name
		return path.Join(ImageFolder, name)
	}

	var total int64
	for _, ref := range pkg.Spine.ItemRefs {
		item, ok := manifest[ref.IDRef]
		if !ok {
			// A spine entry with no manifest item is malformed; skipping it is
			// better than failing an otherwise readable book.
			continue
		}
		if !isDocumentMediaType(item.MediaType) {
			continue
		}
		if hasProperty(item.Properties, "nav") {
			// EPUB 3 navigation documents are usually listed in the spine as
			// well. They are a table of contents, not a chapter.
			continue
		}

		docPath := resolveHref(baseDir, item.Href)
		data, err := readZipEntryLimit(files, docPath, maxDocumentBytes)
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

		// The resolver is per document because an href is relative to the
		// document that wrote it. Copying opts keeps that scoped to this
		// iteration rather than leaking into the next document.
		docOpts := opts
		if opts.KeepImages {
			docDir := path.Dir(docPath)
			docOpts.imageTarget = func(href string) string { return keepImage(docDir, href) }
		}

		headingTitle, text, imageHrefs := documentToChapter(data, docOpts)

		// Accumulate before the empty-document skip below. A cover page flattens
		// to nothing and is skipped as a chapter, but its illustration is only
		// harmless when it is the cover actually stored; if cover detection came
		// up empty, that image was lost like any other.
		//
		// Only references that resolve to an entry in the archive count. A stale
		// link, an external URL and a data: URI all name artwork this file never
		// carried, so reporting them as lost would be a lie.
		//
		// An illustration this pass kept is not a loss. The walk still has to
		// run: it sees references the flattener never reaches, such as an
		// <image> inside an <svg> canvas, and those really are dropped.
		for _, href := range imageHrefs {
			if image, ok := lookupZipEntry(files, resolveHref(path.Dir(docPath), href)); ok {
				if _, kept := keptImages[image]; kept {
					continue
				}
				droppedImages[image] = struct{}{}
			}
		}

		// The table of contents is the authoritative chapter name; the in-document
		// heading is only a fallback for books that ship no usable TOC.
		title := titles[docPath]
		if title == "" {
			title = headingTitle
		}
		if strings.TrimSpace(text) == "" && strings.TrimSpace(title) == "" {
			// Navigation pages and empty separators carry nothing to read.
			continue
		}

		book.Chapters = append(book.Chapters, Chapter{Title: title, Text: text})
	}

	if len(book.Chapters) == 0 {
		return nil, util.NewError("epub contains no readable chapters")
	}

	// The cover survives the import, so it is not a loss to report. Resolving it
	// to its archive entry is what makes the exclusion hold when the manifest
	// and the cover page spell the same file differently.
	if cover, ok := lookupZipEntry(files, coverPath); ok {
		delete(droppedImages, cover)
	}
	book.DroppedImages = len(droppedImages)

	return book, nil
}

// indexZip maps cleaned entry names to their zip files, plus a lower-cased
// fallback index. EPUB paths are meant to be case-sensitive, but archives
// produced by careless tooling disagree with their own manifests.
func indexZip(zr *zip.Reader) map[string]*zip.File {
	files := make(map[string]*zip.File, len(zr.File)*2)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(f.Name)
		files[name] = f
		lower := strings.ToLower(name)
		if _, exists := files[lower]; !exists {
			files[lower] = f
		}
	}
	return files
}

func lookupZipEntry(files map[string]*zip.File, name string) (*zip.File, bool) {
	if name == "" {
		return nil, false
	}
	if f, ok := files[path.Clean(name)]; ok {
		return f, true
	}
	f, ok := files[strings.ToLower(path.Clean(name))]
	return f, ok
}

func readZipEntry(files map[string]*zip.File, name string) ([]byte, error) {
	return readZipEntryLimit(files, name, maxDocumentBytes)
}

func readZipEntryLimit(files map[string]*zip.File, name string, limit int64) ([]byte, error) {
	f, ok := lookupZipEntry(files, name)
	if !ok {
		return nil, util.Errorf("entry not found: %s", name)
	}

	rc, err := f.Open()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer rc.Close()

	// Read one byte past the limit so an oversized entry is detected rather than
	// silently truncated.
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if int64(len(data)) > limit {
		return nil, util.Errorf("%w: %s", errZipEntryTooLarge, name)
	}

	return data, nil
}

// findPackagePath resolves the OPF path via META-INF/container.xml, falling back
// to any .opf entry in the archive when the container is missing or unhelpful.
func findPackagePath(files map[string]*zip.File) (string, error) {
	if data, err := readZipEntry(files, "META-INF/container.xml"); err == nil {
		var container struct {
			RootFiles []struct {
				FullPath  string `xml:"full-path,attr"`
				MediaType string `xml:"media-type,attr"`
			} `xml:"rootfiles>rootfile"`
		}
		if err := xml.Unmarshal(data, &container); err == nil {
			for _, rf := range container.RootFiles {
				full := strings.TrimSpace(rf.FullPath)
				if full == "" {
					continue
				}
				if _, ok := lookupZipEntry(files, full); ok {
					return path.Clean(full), nil
				}
			}
		}
	}

	for name := range files {
		if strings.HasSuffix(strings.ToLower(name), ".opf") {
			return name, nil
		}
	}

	return "", util.NewError("epub has no package document")
}

func collectIdentifiers(ids []opfIdentifier) map[string]string {
	out := make(map[string]string)
	for _, id := range ids {
		value := strings.TrimSpace(id.Value)
		if value == "" {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(id.Scheme))
		if key == "" {
			// urn:isbn:… and urn:uuid:… carry the scheme in the value itself.
			if rest, ok := strings.CutPrefix(strings.ToLower(value), "urn:"); ok {
				if scheme, _, found := strings.Cut(rest, ":"); found {
					key = scheme
				}
			}
		}
		if key == "" {
			key = "identifier"
		}

		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// readCover resolves the cover image through the EPUB 3 manifest property
// first, then the EPUB 2 <meta name="cover"> convention, then a filename guess.
//
// The resolved archive path is returned alongside the bytes so callers can tell
// the cover apart from the illustrations that conversion drops.
func readCover(files map[string]*zip.File, manifest map[string]opfItem, pkg opfPackage, baseDir string) ([]byte, string, string) {
	var candidates []string

	for _, item := range pkg.Manifest.Items {
		if hasProperty(item.Properties, "cover-image") {
			candidates = append(candidates, item.Href)
		}
	}

	for _, meta := range pkg.Metadata.Metas {
		if strings.EqualFold(strings.TrimSpace(meta.Name), "cover") {
			if item, ok := manifest[strings.TrimSpace(meta.Content)]; ok {
				candidates = append(candidates, item.Href)
			}
		}
	}

	for _, item := range pkg.Manifest.Items {
		if !strings.HasPrefix(item.MediaType, "image/") {
			continue
		}
		if strings.Contains(strings.ToLower(item.ID), "cover") ||
			strings.Contains(strings.ToLower(item.Href), "cover") {
			candidates = append(candidates, item.Href)
		}
	}

	for _, href := range candidates {
		coverPath := resolveHref(baseDir, href)
		data, err := readZipEntry(files, coverPath)
		if err != nil || len(data) == 0 {
			continue
		}
		return data, strings.ToLower(path.Ext(coverPath)), coverPath
	}

	return nil, "", ""
}

// readTOCTitles returns chapter titles keyed by resolved document path, reading
// the EPUB 3 navigation document when present and falling back to the EPUB 2
// NCX. An EPUB with neither is fine; chapters then fall back to their own
// headings.
func readTOCTitles(files map[string]*zip.File, manifest map[string]opfItem, pkg opfPackage, baseDir string) map[string]string {
	titles := make(map[string]string)

	for _, item := range pkg.Manifest.Items {
		if !hasProperty(item.Properties, "nav") {
			continue
		}
		navPath := resolveHref(baseDir, item.Href)
		data, err := readZipEntry(files, navPath)
		if err != nil {
			continue
		}
		for href, title := range parseNavDocument(data) {
			titles[resolveHref(path.Dir(navPath), href)] = title
		}
	}

	if len(titles) > 0 {
		return titles
	}

	ncxItem, ok := manifest[strings.TrimSpace(pkg.Spine.TOC)]
	if !ok {
		for _, item := range pkg.Manifest.Items {
			if strings.Contains(item.MediaType, "dtbncx") || strings.HasSuffix(strings.ToLower(item.Href), ".ncx") {
				ncxItem = item
				ok = true
				break
			}
		}
	}
	if !ok {
		return titles
	}

	ncxPath := resolveHref(baseDir, ncxItem.Href)
	data, err := readZipEntry(files, ncxPath)
	if err != nil {
		return titles
	}

	var ncx struct {
		NavPoints []ncxNavPoint `xml:"navMap>navPoint"`
	}
	if err := xml.Unmarshal(data, &ncx); err != nil {
		return titles
	}

	ncxDir := path.Dir(ncxPath)
	var walk func(points []ncxNavPoint)
	walk = func(points []ncxNavPoint) {
		for _, p := range points {
			title := strings.TrimSpace(p.Label.Text)
			src := strings.TrimSpace(p.Content.Src)
			if title != "" && src != "" {
				key := resolveHref(ncxDir, src)
				if _, exists := titles[key]; !exists {
					titles[key] = title
				}
			}
			walk(p.Children)
		}
	}
	walk(ncx.NavPoints)

	return titles
}

// resolveHref turns a manifest or TOC href into a zip entry path. Fragments are
// dropped and percent-encoding is decoded, because zip entry names are stored
// undecoded while hrefs are URLs.
func resolveHref(baseDir, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if idx := strings.IndexByte(href, '#'); idx >= 0 {
		href = href[:idx]
	}
	if href == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(href); err == nil {
		href = decoded
	}
	if after, ok := strings.CutPrefix(href, "/"); ok {
		return path.Clean(after)
	}
	if baseDir == "" || baseDir == "." {
		return path.Clean(href)
	}
	return path.Clean(path.Join(baseDir, href))
}

func hasProperty(properties, want string) bool {
	for p := range strings.FieldsSeq(properties) {
		if strings.EqualFold(p, want) {
			return true
		}
	}
	return false
}

// isDocumentMediaType reports whether a manifest item is spine content. An empty
// media type is accepted because some EPUBs omit it, and the spine already told
// us the item is meant to be read.
func isDocumentMediaType(mediaType string) bool {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case mediaType == "":
		return true
	case strings.Contains(mediaType, "xhtml"):
		return true
	case strings.Contains(mediaType, "text/html"):
		return true
	default:
		return false
	}
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func nonEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// --- package document types ---
//
// Element names are matched on their local name only, without a namespace, so
// that EPUBs using unusual prefixes still decode.

type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest struct {
		Items []opfItem `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		TOC      string       `xml:"toc,attr"`
		ItemRefs []opfItemRef `xml:"itemref"`
	} `xml:"spine"`
}

type opfMetadata struct {
	Titles       []string        `xml:"title"`
	Creators     []string        `xml:"creator"`
	Languages    []string        `xml:"language"`
	Descriptions []string        `xml:"description"`
	Dates        []string        `xml:"date"`
	Identifiers  []opfIdentifier `xml:"identifier"`
	Metas        []opfMeta       `xml:"meta"`
}

type opfIdentifier struct {
	Scheme string `xml:"scheme,attr"`
	Value  string `xml:",chardata"`
}

type opfMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfItemRef struct {
	IDRef string `xml:"idref,attr"`
}

type ncxNavPoint struct {
	Label struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []ncxNavPoint `xml:"navPoint"`
}
