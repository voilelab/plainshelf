package fingerprint

import (
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/testutil"
)

// TestCacheFileMarshalsDeterministically guards writeFile's "the encoded file
// equals the one already on disk, so skip the write" check. Index and Entries
// are maps: without json.Deterministic the comparison never matches and every
// scan rewrites app/fingerprint-cache.json.
func TestCacheFileMarshalsDeterministically(t *testing.T) {
	seen := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	index := map[string]indexEntry{}
	entries := map[string]Entry{}
	for _, name := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"} {
		index["books/"+name+".bookpkg/sources/"+name+"/content.txt"] = indexEntry{
			Size:    int64(len(name)),
			ModTime: seen,
			MD5:     "md5-" + name,
			SeenAt:  seen,
		}
		entries["md5-"+name] = Entry{
			NormMD5:   "norm-" + name,
			NormChars: len(name),
			Shingles:  len(name) * 2,
			Sketch:    "sketch-" + name,
		}
	}

	testutil.AssertMarshalIsDeterministic(t, "fingerprint.cacheFile", cacheFile{
		SchemaVersion: schemaVersion,
		Generator:     "plainshelf/test",
		Algo:          testAlgo,
		Index:         index,
		Entries:       entries,
	}, jsonopt.DiskCompact())
}
