package scancache

import (
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/testutil"
)

// TestScanCacheMarshalsDeterministically covers both places the directory scan
// cache marshals its Dirs map: the file itself, and scanCacheDigest, whose
// whole job is to answer "did anything change?" — a digest that moves on its
// own rewrites app/scan-cache.json after every walk.
func TestScanCacheMarshalsDeterministically(t *testing.T) {
	modTime := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	dirs := map[string]dirSnapshot{}
	for _, name := range []string{"Fiction", "Fiction/Classics", "Nonfiction", "Poetry", "Drama", "Essays", "Letters", "Travel", "History", "Science"} {
		dirs["books/"+name] = dirSnapshot{
			ModTime:  modTime,
			Children: []DirChild{{Name: "a.bookpkg"}, {Name: "b.bookpkg", Symlink: true}},
		}
	}

	testutil.AssertMarshalIsDeterministic(t, "scancache.scanCacheFile", scanCacheFile{
		SchemaVersion: schemaVersion,
		Generator:     "plainshelf/test",
		Dirs:          dirs,
	}, jsonopt.DiskCompact())

	testutil.AssertMarshalIsDeterministic(t, "scancache digest payload", dirs, jsonopt.DiskCompact())
}
