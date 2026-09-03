package readingprogress

import (
	"testing"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/testutil"
)

// TestDocumentMarshalsDeterministically covers stored reading progress, which
// nests a map inside a map: one shelf ID to one book ID to its entry.
func TestDocumentMarshalsDeterministically(t *testing.T) {
	doc := New()
	for _, shelfID := range []string{"shelf-a", "shelf-b", "shelf-c"} {
		books := map[string]Entry{}
		for _, bookID := range []string{"a1", "b2", "c3", "d4", "e5", "f6", "g7", "h8", "i9", "j10"} {
			books[bookID] = Entry{}
		}
		doc.Shelves[shelfID] = books
	}

	testutil.AssertMarshalIsDeterministic(t, "readingprogress.Document", doc, jsonopt.DiskCompact())
}
