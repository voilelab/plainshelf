package bookpkg

import (
	"errors"
	"io/fs"
	"path"

	"github.com/voilelab/plainshelf/internal/util"
)

func (r *Source) SizeBytes() (int64, error) {
	info, err := r.root.Stat(path.Join(r.folderPath, SourceFile))
	if err != nil {
		return 0, util.Errorf("%w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return 0, util.Errorf("invalid source file size")
	}
	assets, err := r.assetSize(path.Join(r.folderPath, SourceAssetsFolder))
	if err != nil {
		return 0, err
	}
	return info.Size() + assets, nil
}

func (r *Source) assetSize(dir string) (int64, error) {
	entries, err := r.root.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, util.Errorf("%w", err)
	}
	var size int64
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if entry.IsDir() {
			nested, err := r.assetSize(path.Join(dir, entry.Name()))
			if err != nil {
				return 0, err
			}
			size += nested
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, util.Errorf("%w", err)
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 {
				return 0, util.Errorf("invalid asset size")
			}
			size += info.Size()
		}
	}
	return size, nil
}
