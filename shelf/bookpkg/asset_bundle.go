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

// WriteAssetsZip adds illustrations under their flat names, skipping absent
// files. The caller owns the writer. A remote ZipOpener saves round trips;
// entries are still repacked with Store, just as for individual asset reads.
func (r *Source) WriteAssetsZip(zw *zip.Writer, names []string) error {
	for _, name := range names {
		if _, err := r.AssetPath(name); err != nil {
			return err
		}
	}
	if opener, ok := r.root.(fsutil.ZipOpener); ok {
		if handled, err := r.writeAssetBatch(zw, opener, names); handled {
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
		err = writeZipAsset(zw, name, asset.Info, asset.File)
		_ = asset.File.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type bundleAsset struct {
	name string
	info fs.FileInfo
}

func (r *Source) writeAssetBatch(zw *zip.Writer, opener fsutil.ZipOpener, names []string) (bool, error) {
	assets := make([]bundleAsset, 0, len(names))
	paths := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		assetPath, err := r.AssetPath(name)
		if err != nil {
			return true, err
		}
		info, err := r.root.Stat(assetPath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return true, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		assets = append(assets, bundleAsset{name: name, info: info})
		if !seen[assetPath] {
			paths = append(paths, assetPath)
			seen[assetPath] = true
		}
	}
	if len(paths) == 0 {
		return true, nil
	}
	body, err := opener.OpenZip(paths...)
	if errors.Is(err, errors.ErrUnsupported) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	defer body.Close() //nolint:errcheck // read failures are returned below

	// archive/zip requires ReaderAt. Spool the compressed archive to disk so
	// hand-placed, arbitrarily large illustrations do not fill memory.
	tmp, err := os.CreateTemp("", "plainshelf-assets-*.zip")
	if err != nil {
		return true, err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	size, err := io.Copy(tmp, body)
	if err != nil {
		return true, err
	}
	zr, err := zip.NewReader(tmp, size)
	if err != nil {
		return true, err
	}
	entries := make(map[string]*zip.File, len(zr.File))
	for _, entry := range zr.File {
		if _, exists := entries[entry.Name]; exists {
			return true, util.NewError("duplicate asset in remote zip")
		}
		entries[entry.Name] = entry
	}
	for _, asset := range assets {
		entry, ok := entries[asset.name]
		if !ok {
			return true, util.Errorf("asset %q missing from remote zip", asset.name)
		}
		file, err := entry.Open()
		if err != nil {
			return true, err
		}
		err = writeZipAsset(zw, asset.name, asset.info, file)
		_ = file.Close()
		if err != nil {
			return true, err
		}
	}
	return true, nil
}

func writeZipAsset(zw *zip.Writer, name string, info fs.FileInfo, file io.Reader) error {
	entry, err := zw.CreateHeader(&zip.FileHeader{
		Name: name, Method: zip.Store, Modified: info.ModTime(),
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}
