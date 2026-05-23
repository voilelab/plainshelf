package shelf

import (
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// FileStat is the last modified time and size of the book meta,
// used for cache validation, and should be updated whenever the book meta is updated
type FileStat struct {
	ModTime time.Time
	Size    int64
}

func (f *FileStat) Equal(other *FileStat) bool {
	return f.ModTime.Equal(other.ModTime) && f.Size == other.Size
}

func getFileStat(rt fsutil.FS, path string) (*FileStat, error) {
	info, err := rt.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileStat{
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, nil
}
