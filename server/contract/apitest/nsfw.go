package apitest

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

// The fixture for show_nsfw, the setting that decides whether this server serves
// the books its shelves mark as adult content. It is here rather than in one
// contract package because the filter is not one area's rule: books, folders,
// sources and the book cache are each asserted against the same shelf.
//
//	books/
//	├─ Visible                 plainly visible
//	└─ Fiction/
//	   ├─ Classics/Classic     visible, so Fiction survives with it
//	   ├─ Adult/FolderHidden   hidden by the shelf.json folder rule
//	   ├─ Flagged/BookHidden   hidden by its own book.json nsfw
//	   └─ Empty/               never held a book, and must stay listed
//
// Fiction/Flagged is the case a folder rule cannot reach: it is not marked, so
// it is only empty once its one book is filtered out.
const (
	NSFWMarkedFolder  = "Fiction/Adult"
	NSFWFlaggedFolder = "Fiction/Flagged"
	NSFWEmptyFolder   = "Fiction/Empty"

	// NSFWMarkedReason is carried by the rule so a test can assert the note the
	// user wrote reaches the client, which is what an editor shows in place of a
	// checkbox it must not offer.
	NSFWMarkedReason = "kept apart from the rest of the shelf"

	nsfwShelfJSON = `{"schema_version":1,"content":{"nsfw_folders":[{"path":"` + NSFWMarkedFolder +
		`","reason":"` + NSFWMarkedReason + `"}]}}`
)

// Visible and FolderHidden are given this same body, so they are a duplicate
// pair and — it is long enough to yield many distinct shingles — a similar pair
// too. Either endpoint has something to report only when the hidden half is
// served, which is the assertion those areas make.
const nsfwTwinProse = "the quick brown fox jumps over the lazy dog while the industrious " +
	"bee gathers nectar from a thousand summer flowers and the river winds " +
	"slowly toward the distant glimmering sea beneath an indifferent sky " +
	"the quick brown fox jumps over the lazy dog while the industrious " +
	"bee gathers nectar from a thousand summer flowers and the river winds " +
	"slowly toward the distant glimmering sea beneath an indifferent sky "

// NSFWShelf is the fixture and its four book IDs, named for why each one is or
// is not served.
type NSFWShelf struct {
	Env *Env

	Visible, Classic, FolderHidden, BookHidden string
}

// Hidden is what a request with show_nsfw off must not see, and All is the whole
// shelf, which is what it sees with the setting on.
func (s NSFWShelf) Hidden() []string { return []string{s.FolderHidden, s.BookHidden} }
func (s NSFWShelf) All() []string {
	return []string{s.Visible, s.Classic, s.FolderHidden, s.BookHidden}
}

// NewNSFWShelf builds it. shelf.json is written before the app opens because it
// is read once at open; the book's own mark goes straight into book.json and is
// rescanned, which covers the other way a mark arrives - the file is the source
// of truth, and a shelf may be marked with a text editor. SetBookNSFW is the
// route a user takes, and has its own test.
func NewNSFWShelf(t *testing.T) NSFWShelf {
	t.Helper()

	libRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(libRoot, "shelf.json"), []byte(nsfwShelfJSON), 0o644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}
	env := New(t, WithLibRoot(libRoot))

	book := func(title, folder, body string) string {
		return ImportTextBook(t, env, title, folder, title+".txt", body).Meta.ID
	}
	shelf := NSFWShelf{
		Env:          env,
		Visible:      book("Visible", "", nsfwTwinProse),
		Classic:      book("Classic", "Fiction/Classics", "unrelated vocabulary weaving mercury zephyr quartz lantern"),
		FolderHidden: book("FolderHidden", NSFWMarkedFolder, nsfwTwinProse),
		BookHidden:   book("BookHidden", NSFWFlaggedFolder, "another vocabulary entirely obsidian marigold trestle vellum"),
	}

	AssertStatus(t, env.Post(ShelfURL("folders", NSFWEmptyFolder), nil), http.StatusNoContent)
	EditBookMetaFile(t, env, shelf.BookHidden, map[string]any{"nsfw": true})
	AssertStatus(t, env.Post(ScansURL(), nil), http.StatusOK)
	return shelf
}

// SetBookNSFW writes a book's own half of the mark through the PATCH route, the
// way the metadata editor does.
func SetBookNSFW(t *testing.T, env *Env, bookID string, nsfw bool) server.Book {
	t.Helper()

	rec := env.Patch(BookURL(bookID), strings.NewReader(`{"nsfw":`+strconv.FormatBool(nsfw)+`}`))
	AssertStatus(t, rec, http.StatusOK)
	return DecodeJSON[server.Book](t, rec)
}

// SetShowNSFW flips the setting the way the settings page does.
func SetShowNSFW(t *testing.T, env *Env, show bool) {
	t.Helper()

	rec := env.Post(SettingURL("show_nsfw"), strings.NewReader(strconv.FormatBool(show)))
	AssertStatus(t, rec, http.StatusNoContent)
}
