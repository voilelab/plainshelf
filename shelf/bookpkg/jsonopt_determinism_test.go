package bookpkg

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/testutil"
)

// TestBookMetaMarshalsDeterministically covers book.json, whose Identifiers is
// the one map a reader can grow by hand. A book.json that reshuffles on every
// write turns an untouched shelf into a diff in the user's backup.
func TestBookMetaMarshalsDeterministically(t *testing.T) {
	meta := BookMeta{
		SchemaVersion: BookMetaSchemaVersion,
		ID:            "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
		Title:         "The Tale of Genji",
		Format:        "txt",
		Tags:          []string{"classic", "japanese"},
		Identifiers: map[string]string{
			"isbn":      "978-0-14-243714-8",
			"douban":    "1234567",
			"oclc":      "123456789",
			"goodreads": "42",
			"lccn":      "n78-089035",
			"asin":      "B000FC0PDA",
			"openlib":   "OL123M",
			"worldcat":  "987654321",
			"gbooks":    "zyTCAlFPjgYC",
			"jpno":      "21234567",
		},
		Authors:       []string{"Murasaki Shikibu"},
		Language:      "ja",
		CurrentSource: "src-1",
	}

	testutil.AssertMarshalIsDeterministic(t, "bookpkg.BookMeta", meta, jsonopt.Disk())
}
