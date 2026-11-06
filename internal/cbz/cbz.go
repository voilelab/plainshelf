package cbz

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"
	"slices"
	"strings"

	_ "golang.org/x/image/webp"
)

const (
	maxEntryBytes = 20 << 20
	maxTotalBytes = 2 << 30
	maxEntries    = 2000
	maxPixels     = 100_000_000
)

type Page struct {
	Name  string
	entry *zip.File
}

type Book struct {
	Title     string
	Authors   []string
	Direction string
	Pages     []Page
}

type comicInfo struct {
	Title  string
	Writer string
	Manga  string
}

func Open(r io.ReaderAt, size int64) (*Book, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("not a readable cbz archive: %w", err)
	}
	if len(zr.File) > maxEntries {
		return nil, fmt.Errorf("cbz exceeds 2000 entry limit")
	}
	var total uint64
	for _, entry := range zr.File {
		if !safePath(entry.Name) {
			return nil, fmt.Errorf("cbz path traversal or absolute path: %q", entry.Name)
		}
		if !entry.Mode().IsRegular() && !entry.FileInfo().IsDir() {
			return nil, fmt.Errorf("cbz entry %q is not a regular file", entry.Name)
		}
		if entry.UncompressedSize64 > maxEntryBytes {
			return nil, fmt.Errorf("cbz entry %q exceeds 20 MiB limit", entry.Name)
		}
		total += entry.UncompressedSize64
		if total > maxTotalBytes {
			return nil, fmt.Errorf("cbz exceeds 2 GiB total uncompressed size limit")
		}
	}
	entries := slices.Clone(zr.File)
	slices.SortStableFunc(entries, func(a, b *zip.File) int { return naturalCompare(a.Name, b.Name) })
	book := &Book{}
	for _, entry := range entries {
		if entry.FileInfo().IsDir() || hiddenPath(entry.Name) {
			continue
		}
		if strings.EqualFold(path.Base(entry.Name), "ComicInfo.xml") {
			data, err := readEntry(entry)
			if err != nil {
				return nil, err
			}
			var info comicInfo
			if err := xml.Unmarshal(data, &info); err != nil {
				return nil, fmt.Errorf("invalid ComicInfo.xml: %w", err)
			}
			book.Title = strings.TrimSpace(info.Title)
			for author := range strings.SplitSeq(info.Writer, ",") {
				if author = strings.TrimSpace(author); author != "" {
					book.Authors = append(book.Authors, author)
				}
			}
			if strings.TrimSpace(info.Manga) == "YesAndRightToLeft" {
				book.Direction = "rtl"
			}
			continue
		}
		ext := path.Ext(entry.Name)
		switch strings.ToLower(ext) {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif":
			book.Pages = append(book.Pages, Page{Name: rand.Text() + ext, entry: entry})
		}
	}
	if len(book.Pages) == 0 {
		return nil, fmt.Errorf("cbz contains no supported images")
	}
	return book, nil
}

func (p Page) Read() ([]byte, error) {
	data, err := readEntry(p.entry)
	if err != nil {
		return nil, err
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid cbz image %q: %w", p.entry.Name, err)
	}
	if config.Width <= 0 || config.Height <= 0 || uint64(config.Width) > maxPixels/uint64(config.Height) {
		return nil, fmt.Errorf("cbz image %q exceeds 100 MP pixel limit", p.entry.Name)
	}
	return data, nil
}

func readEntry(entry *zip.File) ([]byte, error) {
	r, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("read cbz entry %q: %w", entry.Name, err)
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxEntryBytes+1))
	if len(data) > maxEntryBytes {
		return nil, fmt.Errorf("cbz entry %q exceeds 20 MiB limit", entry.Name)
	}
	if err != nil {
		return nil, fmt.Errorf("read cbz entry %q: %w", entry.Name, err)
	}
	return data, nil
}

func safePath(name string) bool {
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return false
	}
	for part := range strings.SplitSeq(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func hiddenPath(name string) bool {
	for part := range strings.SplitSeq(strings.ReplaceAll(name, `\`, "/"), "/") {
		if strings.HasPrefix(part, ".") || part == "__MACOSX" {
			return true
		}
	}
	return false
}

func naturalCompare(a, b string) int {
	originalA, originalB := a, b
	a, b = strings.ToLower(a), strings.ToLower(b)
	for len(a) > 0 && len(b) > 0 {
		if digit(a[0]) && digit(b[0]) {
			i, j := 0, 0
			for i < len(a) && digit(a[i]) {
				i++
			}
			for j < len(b) && digit(b[j]) {
				j++
			}
			x, y := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(x) != len(y) {
				if len(x) < len(y) {
					return -1
				}
				return 1
			}
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
			a, b = a[i:], b[j:]
		} else {
			if a[0] != b[0] {
				if a[0] < b[0] {
					return -1
				}
				return 1
			}
			a, b = a[1:], b[1:]
		}
	}
	if c := strings.Compare(a, b); c != 0 {
		return c
	}
	return strings.Compare(originalA, originalB)
}

func digit(c byte) bool { return c >= '0' && c <= '9' }
