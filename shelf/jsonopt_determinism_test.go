package shelf

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/testutil"
)

// bookCacheDeterminismFixture builds a Books map wide enough that an unsorted
// marshal would reorder it.
func bookCacheDeterminismFixture() (folders []string, books map[string]BookCacheEntry) {
	folders = []string{"/", "Fiction", "Fiction/Classics", "Nonfiction"}
	books = map[string]BookCacheEntry{}
	for _, id := range []string{"a1", "b2", "c3", "d4", "e5", "f6", "g7", "h8", "i9", "j10"} {
		books[id] = BookCacheEntry{
			Path: "books/Fiction/" + id + ".bookpkg",
			Meta: &BookMeta{
				SchemaVersion: BookMetaSchemaVersion,
				ID:            id,
				Title:         "Book " + id,
				Authors:       []string{"Author " + id},
				Identifiers:   map[string]string{"isbn": "isbn-" + id, "douban": "douban-" + id, "oclc": "oclc-" + id},
			},
		}
	}
	return folders, books
}

// TestBookCacheFileMarshalsDeterministically guards the exported book cache,
// which is written only when bookCacheDigest changes. On a shelf held on pCloud
// or SMB an identical rewrite is a pointless upload, and Books is a map.
func TestBookCacheFileMarshalsDeterministically(t *testing.T) {
	folders, books := bookCacheDeterminismFixture()

	testutil.AssertMarshalIsDeterministic(t, "shelf.BookCacheFile", BookCacheFile{
		SchemaVersion: BookCacheSchemaVersion,
		WriterID:      "test-writer",
		Timestamp:     1_772_000_000,
		Generator:     "plainshelf/test",
		Folders:       folders,
		Books:         books,
	}, jsonopt.Disk())

	// The digest payload is a narrower struct than the file, so it needs its
	// own assertion; keep it in step with bookCacheDigest.
	testutil.AssertMarshalIsDeterministic(t, "shelf book cache digest payload", struct {
		Folders []string                  `json:"folders"`
		Books   map[string]BookCacheEntry `json:"books"`
	}{Folders: folders, Books: books}, jsonopt.DiskCompact())
}
