package bookpkg

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

func (r *Source) WriteAssetsToZip(zw *zip.Writer, names []string) error {
	if len(names) == 0 {
		return nil
	}
	if z, ok := r.root.(fsutil.ZipOpener); ok {
		err := r.writeBatchAssets(zw, z, names)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
	}
	for _, name := range names {
		asset, err := r.OpenAsset(name)
		if errors.Is(err, ErrAssetNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		err = writeBundleEntry(zw, name, asset.Info, asset.File)
		closeErr := asset.File.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func (r *Source) writeBatchAssets(zw *zip.Writer, z fsutil.ZipOpener, names []string) error {
	paths := make([]string, 0, len(names))
	infos := make(map[string]fs.FileInfo, len(names))
	for _, name := range names {
		p, err := r.AssetPath(name)
		if err != nil {
			return err
		}
		info, err := r.root.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if _, exists := infos[name]; !exists {
			paths = append(paths, p)
			infos[name] = info
		}
	}
	if len(paths) == 0 {
		return nil
	}
	body, err := z.OpenZip(paths...)
	if err != nil {
		return err
	}
	defer body.Close()

	// zip.Reader needs ReaderAt. Spool the compressed archive to disk rather
	// than retaining every illustration in memory; no files are extracted.
	tmp, err := os.CreateTemp("", "plainshelf-assets-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	size, err := io.Copy(tmp, body)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(tmp, size)
	if err != nil {
		return err
	}
	entries := make(map[string]*zip.File, len(zr.File))
	for _, entry := range zr.File {
		entries[entry.Name] = entry
	}
	for _, name := range names {
		info, exists := infos[name]
		if !exists {
			continue
		}
		entry, exists := entries[name]
		if !exists || !entry.Mode().IsRegular() {
			return util.Errorf("asset %q missing from batch zip", name)
		}
		file, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeBundleEntry(zw, name, info, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func writeBundleEntry(zw *zip.Writer, name string, info fs.FileInfo, file io.Reader) error {
	entry, err := zw.CreateHeader(&zip.FileHeader{
		Name: name, Method: zip.Store, Modified: info.ModTime(),
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}
