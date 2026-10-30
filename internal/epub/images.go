package epub

import (
	"archive/zip"
	"fmt"
	"path"
	"strings"
)

type imageCollector struct {
	files      map[string]*zip.File
	kept       map[*zip.File]string
	dropped    map[*zip.File]struct{}
	images     []Image
	totalBytes int64
}

func newImageCollector(files map[string]*zip.File) *imageCollector {
	return &imageCollector{
		files:   files,
		kept:    make(map[*zip.File]string),
		dropped: make(map[*zip.File]struct{}),
	}
}

func (c *imageCollector) keep(docDir, href string) string {
	entry, ok := lookupZipEntry(c.files, resolveHref(docDir, href))
	if !ok {
		return ""
	}
	if name, ok := c.kept[entry]; ok {
		return path.Join(ImageFolder, name)
	}
	if _, dropped := c.dropped[entry]; dropped {
		return ""
	}

	ext := strings.ToLower(path.Ext(entry.Name))
	if !imageExtensions[ext] {
		c.markDropped(entry)
		return ""
	}

	data, err := readZipEntryLimit(c.files, entry.Name, maxImageBytes)
	if err != nil || c.totalBytes+int64(len(data)) > maxTotalImageBytes {
		c.markDropped(entry)
		return ""
	}
	c.totalBytes += int64(len(data))

	name := fmt.Sprintf("img-%04d%s", len(c.images)+1, ext)
	c.images = append(c.images, Image{Name: name, Data: data})
	c.kept[entry] = name
	return path.Join(ImageFolder, name)
}

func (c *imageCollector) markDropped(entry *zip.File) {
	c.dropped[entry] = struct{}{}
}

func (c *imageCollector) recordUnflattenedRefs(docDir string, hrefs []string) {
	for _, href := range hrefs {
		if image, ok := lookupZipEntry(c.files, resolveHref(docDir, href)); ok {
			if _, kept := c.kept[image]; kept {
				continue
			}
			c.markDropped(image)
		}
	}
}
