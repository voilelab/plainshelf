package bookpkg

import (
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// FileStat is what a cache keyed on "has this file changed" compares.
type FileStat struct {
	ModTime time.Time
	Size    int64
}

func (f *FileStat) Equal(other *FileStat) bool {
	return f.ModTime.Equal(other.ModTime) && f.Size == other.Size
}

func getFileStat(rt fsutil.ReadFS, path string) (*FileStat, error) {
	info, err := rt.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileStat{
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, nil
}
