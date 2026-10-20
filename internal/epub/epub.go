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

const (
	maxDocumentBytes = 32 << 20
	maxTotalBytes    = 256 << 20

	maxImageBytes      = 8 << 20
	maxTotalImageBytes = 64 << 20
)

var errZipEntryTooLarge = errors.New("zip entry exceeds the maximum supported size")

const ImageFolder = "assets"

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

type Chapter struct {
	Title string

	Text string
}

type Image struct {
	Name string
	Data []byte
}

type Book struct {
	Title       string
	Authors     []string
	Language    string
	Description string

	Identifiers map[string]string

	// PublishedAt is the raw dc:date string. EPUB does not constrain the format
	// beyond a recommendation, so parsing is left to the caller.
	PublishedAt string

	Cover    []byte
	CoverExt string

	Images []Image

	DroppedImages int

	Chapters []Chapter
}

type Options struct {
	MarkdownInline bool

	KeepImages bool

	imageTarget func(href string) string
}

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

	droppedImages := make(map[*zip.File]struct{})

	keptImages := make(map[*zip.File]string)
	var imageBytes int64

	keepImage := func(docDir, href string) string {
		entry, ok := lookupZipEntry(files, resolveHref(docDir, href))
		if !ok {
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
			continue
		}
		if !isDocumentMediaType(item.MediaType) {
			continue
		}
		if hasProperty(item.Properties, "nav") {
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

		docOpts := opts
		if opts.KeepImages {
			docDir := path.Dir(docPath)
			docOpts.imageTarget = func(href string) string { return keepImage(docDir, href) }
		}

		headingTitle, text, imageHrefs := documentToChapter(data, docOpts)

		for _, href := range imageHrefs {
			if image, ok := lookupZipEntry(files, resolveHref(path.Dir(docPath), href)); ok {
				if _, kept := keptImages[image]; kept {
					continue
				}
				droppedImages[image] = struct{}{}
			}
		}

		title := titles[docPath]
		if title == "" {
			title = headingTitle
		}
		if strings.TrimSpace(text) == "" && strings.TrimSpace(title) == "" {
			continue
		}

		book.Chapters = append(book.Chapters, Chapter{Title: title, Text: text})
	}

	if len(book.Chapters) == 0 {
		return nil, util.NewError("epub contains no readable chapters")
	}

	if cover, ok := lookupZipEntry(files, coverPath); ok {
		delete(droppedImages, cover)
	}
	book.DroppedImages = len(droppedImages)

	return book, nil
}

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

	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if int64(len(data)) > limit {
		return nil, util.Errorf("%w: %s", errZipEntryTooLarge, name)
	}

	return data, nil
}

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
	href, _, _ = strings.Cut(href, "#")
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
