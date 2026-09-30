package shelf

import (
	"encoding/json/v2"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

// The NSFW fixture shelf. The tree is chosen so that every way a rule can go
// wrong is representable: a marked folder, a folder below it, a sibling whose
// name STARTS with the marked one (the prefix trap), and an unmarked folder.
//
//	books/
//	├─ loose-0001.bookpkg              (top level, says nothing)
//	├─ flagged-0001.bookpkg            (top level, nsfw: true)
//	└─ Fiction/
//	   ├─ Classics/clean-0001.bookpkg
//	   ├─ AdultManga/manga-0001.bookpkg
//	   └─ Adult/
//	      ├─ adult-0001.bookpkg
//	      ├─ denied-0001.bookpkg       (nsfw: false)
//	      └─ Deep/adult-0002.bookpkg
var nsfwFixtureBooks = []struct {
	folderDir string
	bookID    string
	nsfwField string
}{
	{"", "loose-0001", ""},
	{"", "flagged-0001", `,"nsfw":true`},
	{"Fiction/Classics", "clean-0001", ""},
	{"Fiction/AdultManga", "manga-0001", ""},
	{"Fiction/Adult", "adult-0001", ""},
	{"Fiction/Adult", "denied-0001", `,"nsfw":false`},
	{"Fiction/Adult/Deep", "adult-0002", ""},
}

// writeNSFWFixtureBook plants a book package on disk with a hand-written
// book.json, so the nsfw member can be present as true, present as false, or
// absent - three states the Go struct alone cannot express.
func writeNSFWFixtureBook(t *testing.T, libRoot, folderDir, bookID, nsfwField string) {
	t.Helper()

	bookPath := path.Join(libRoot, booksFolder, folderDir, bookID+bookExtension)
	if err := os.MkdirAll(bookPath, 0755); err != nil {
		t.Fatalf("Failed to create book dir %s: %v", bookPath, err)
	}
	meta := `{"schema_version":1,"id":"` + bookID + `","title":"` + bookID + `"` + nsfwField + `}`
	if err := os.WriteFile(path.Join(bookPath, "book.json"), []byte(meta), 0644); err != nil {
		t.Fatalf("Failed to write book.json in %s: %v", bookPath, err)
	}
}

// openNSFWShelf builds the fixture shelf above with the given shelf.json, or
// none when body is empty.
func openNSFWShelf(t *testing.T, body string) *Shelf {
	t.Helper()

	libRoot := t.TempDir()
	for _, book := range nsfwFixtureBooks {
		writeNSFWFixtureBook(t, libRoot, book.folderDir, book.bookID, book.nsfwField)
	}
	if body != "" {
		writeShelfConfig(t, libRoot, body)
	}

	return newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
}

// markedBookIDs is what the shelf answers for every book it holds, as a sorted
// list of the IDs it marks. wantCount guards the answer against a listing that
// quietly returned fewer books than the fixture holds, which would otherwise
// read as "nothing is marked".
func markedBookIDs(t *testing.T, s *Shelf, wantCount int) []string {
	t.Helper()

	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	if len(listings) != wantCount {
		t.Fatalf("listed %d books, want %d", len(listings), wantCount)
	}

	var marked []string
	for _, listing := range listings {
		if s.IsBookNSFW(listing.Folders, listing.Book.GetMeta()) {
			marked = append(marked, listing.Book.ID())
		}
	}
	slices.Sort(marked)
	return marked
}

func assertMarked(t *testing.T, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("marked as NSFW %v, want %v", got, want)
	}
}

// A folder rule marks the folder itself and everything below it, and nothing
// beside it. "Fiction/AdultManga" is the case worth pinning: a plain string
// prefix test would match it against "Fiction/Adult" and mark a folder the user
// never named.
func TestNSFWFolderMarksItsWholeSubtree(t *testing.T) {
	s := openNSFWShelf(t, `{
		"schema_version": 1,
		"content": {
			"nsfw_folders": [
				{"path": "Fiction/Adult", "reason": "adult shelf"}
			]
		}
	}`)

	assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)), []string{"adult-0001", "adult-0002", "denied-0001", "flagged-0001"})
}

// A book's own flag marks that one book wherever it sits, with no folder rule in
// play at all.
func TestNSFWBookFlagMarksOneBook(t *testing.T) {
	s := openNSFWShelf(t, "")

	assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)), []string{"flagged-0001"})
}

// The asymmetry the mark depends on: a book may add itself, but it may not take
// itself out of a marked folder. The failure this rules out is a book that
// should have been marked and quietly was not, so nsfw: false on denied-0001
// changes nothing while it sits under a marked folder.
func TestNSFWBookFalseCannotCancelTheFolder(t *testing.T) {
	s := openNSFWShelf(t, `{
		"content": {"nsfw_folders": [{"path": "Fiction/Adult"}]}
	}`)

	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}

	found := false
	for _, listing := range listings {
		if listing.Book.ID() != "denied-0001" {
			continue
		}
		found = true

		meta := listing.Book.GetMeta()
		if meta.NSFW {
			t.Fatal("fixture is wrong: denied-0001 should carry nsfw: false on disk")
		}
		if !s.IsBookNSFW(listing.Folders, meta) {
			t.Error("a book with nsfw: false in a marked folder is not marked; the folder must win")
		}
	}
	if !found {
		t.Fatal("denied-0001 was not listed")
	}
}

// A path is a location, so the three spellings of one location agree. Case
// folds like the ignored-directory names do, for the same reason: a share
// exported over SMB may report either.
func TestNSFWFolderPathSpellings(t *testing.T) {
	want := []string{"adult-0001", "adult-0002", "denied-0001", "flagged-0001"}
	for _, spelling := range []string{
		"Fiction/Adult",
		"fiction/adult",
		"FICTION/ADULT",
		"/Fiction/Adult/",
		"Fiction//Adult",
	} {
		t.Run(spelling, func(t *testing.T) {
			s := openNSFWShelf(t, `{
				"content": {"nsfw_folders": [{"path": `+quoteJSON(t, spelling)+`}]}
			}`)
			assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)), want)
		})
	}
}

// Marking a top-level folder marks everything under it, including the folder
// below it - and still not the books sitting directly in books/.
func TestNSFWTopLevelFolderMarksEverythingUnderIt(t *testing.T) {
	s := openNSFWShelf(t, `{
		"content": {"nsfw_folders": [{"path": "Fiction"}]}
	}`)

	assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)),
		[]string{"adult-0001", "adult-0002", "clean-0001", "denied-0001", "flagged-0001", "manga-0001"})
}

// An entry that cannot name a folder is dropped on its own, the way an unusable
// ignored_dirs entry is: the rest of the list is still what the shelf said. A
// bare string is among them - one entry, one shape - and so is the empty path,
// which would otherwise mark the whole shelf on a typo.
func TestNSFWSkipsUnusableEntries(t *testing.T) {
	s := openNSFWShelf(t, `{
		"content": {
			"nsfw_folders": [
				"Fiction/Adult",
				{"path": ""},
				{"path": "/"},
				{"path": "Fiction/../Adult"},
				{"path": 42},
				{"name": "Fiction/Adult"},
				{"path": "Fiction/AdultManga", "reason": "kept"}
			]
		}
	}`)

	assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)), []string{"flagged-0001", "manga-0001"})
}

// A shelf with no content block behaves exactly as it did before the field
// existed: nothing is marked, and a book.json with no nsfw member reads as a
// book that says nothing.
func TestNSFWAbsentConfigurationMarksNothing(t *testing.T) {
	libRoot := t.TempDir()
	writeNSFWFixtureBook(t, libRoot, "Fiction/Adult", "adult-0001", "")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	assertMarked(t, markedBookIDs(t, s, 1), nil)
}

// The zero value is what a shelf that has said nothing gets: no folder is
// marked, including the top level.
func TestNSFWRulesZeroValue(t *testing.T) {
	var rules shelfutil.NSFWRules

	if rules.IsNSFWFolder(nil) || rules.IsNSFWFolder([]string{"Fiction"}) {
		t.Error("the zero value marks a folder; it must mark nothing")
	}
	if len(rules.Paths()) != 0 {
		t.Errorf("the zero value reports paths %v", rules.Paths())
	}
}

// quoteJSON renders a Go string as a JSON string literal, so a test can build a
// shelf.json body around a path without hand-escaping it.
func quoteJSON(t *testing.T, value string) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal(%q): %v", value, err)
	}
	return string(encoded)
}

// The exported cache is the answer for a client that never reads shelf.json —
// the Android client on a pCloud shelf downloads this one file — so each entry
// carries the assembled answer, not the book's own half of it.
func TestNSFWExportedCacheCarriesTheAssembledAnswer(t *testing.T) {
	libRoot := t.TempDir()
	for _, book := range nsfwFixtureBooks {
		writeNSFWFixtureBook(t, libRoot, book.folderDir, book.bookID, book.nsfwField)
	}
	writeShelfConfig(t, libRoot, `{
		"content": {"nsfw_folders": [{"path": "Fiction/Adult"}]}
	}`)

	newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})
	cache := waitForBookCacheExport(t, libRoot, testWriterID)

	want := map[string]bool{
		"loose-0001":   false,
		"flagged-0001": true,
		"clean-0001":   false,
		"manga-0001":   false,
		"adult-0001":   true,
		"denied-0001":  true,
		"adult-0002":   true,
	}
	if len(cache.Books) != len(want) {
		t.Fatalf("exported %d books, want %d", len(cache.Books), len(want))
	}
	for bookID, wantNSFW := range want {
		entry, ok := cache.Books[bookID]
		if !ok {
			t.Errorf("%s is missing from the exported cache", bookID)
			continue
		}
		if entry.NSFW != wantNSFW {
			t.Errorf("%s exported nsfw = %v, want %v", bookID, entry.NSFW, wantNSFW)
		}
		// The book's own half stays exactly what its book.json says, so a
		// reader can still tell a marked folder from a marked book.
		wantOwn := bookID == "flagged-0001"
		if entry.Meta.NSFW != wantOwn {
			t.Errorf("%s exported meta.nsfw = %v, want %v", bookID, entry.Meta.NSFW, wantOwn)
		}
	}
}

// The reverse case, and the one that has to hold for every shelf in the world
// that has not asked for any of this: with no content block and no book marked,
// the exported file is byte-identical to the one written before the field
// existed. Both new fields are omitted when false, so nothing re-uploads.
func TestNSFWUnmarkedShelfExportsUnchangedBytes(t *testing.T) {
	libRoot := t.TempDir()
	writeNSFWFixtureBook(t, libRoot, "Fiction/Classics", "clean-0001", "")
	newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})
	waitForBookCacheExport(t, libRoot, testWriterID)

	data, err := os.ReadFile(path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix))
	if err != nil {
		t.Fatalf("read the exported cache: %v", err)
	}
	if body := string(data); strings.Contains(body, "nsfw") {
		t.Errorf("an unmarked shelf exported an nsfw key:\n%s", body)
	}
}

// The same for book.json: a book written through the shelf with nothing marked
// carries no nsfw member at all, so an older build rewriting it drops nothing.
func TestNSFWUnmarkedBookWritesNoField(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book, err := s.NewBook(FolderPath{"Fiction"}, "Plain")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	data, err := os.ReadFile(path.Join(libRoot, book.PackagePath(), BookMetaFile))
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}
	if body := string(data); strings.Contains(body, "nsfw") {
		t.Errorf("an unmarked book wrote an nsfw key:\n%s", body)
	}
}

// A marked book survives an ordinary metadata edit, which is what makes the mark
// usable at all: the flag is set once and every later write carries it.
func TestNSFWSurvivesAMetadataWrite(t *testing.T) {
	libRoot := t.TempDir()
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book, err := s.NewBook(FolderPath{"Fiction"}, "Marked")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	meta := book.GetMeta()
	meta.NSFW = true
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	renamed := book.GetMeta()
	renamed.Title = "Marked, renamed"
	if err := book.SetMeta(renamed); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	reopened, err := s.GetBook(book.ID())
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if !reopened.GetMeta().NSFW {
		t.Error("nsfw was lost by a later metadata write")
	}
}

// Case-insensitive matching has to mean what Unicode means by it, not what
// lowercasing happens to do. Both halves of this are a folder rule silently
// missed — a book that should have been marked quietly is not — so both are
// pinned:
//
//   - Greek final sigma: "Σ" lowercases to "σ" and "ς" lowercases to itself, so
//     lowercase keys alone give one letter two spellings that never meet;
//   - Turkish dotted capital I: Unicode's simple case folding leaves "İ" alone,
//     so folding alone loses a match that lowercasing gets right.
func TestNSFWFolderPathCaseFolding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rule  string
		below []string
		want  bool
	}{
		{"capital sigma rule, final sigma folder", "ΣΕΙΡΑΣ", []string{"σειρας"}, true},
		{"final sigma rule, capital sigma folder", "σειρας", []string{"ΣΕΙΡΑΣ"}, true},
		{"turkish dotted capital", "İstanbul", []string{"istanbul"}, true},
		{"turkish lowercase", "istanbul", []string{"İstanbul"}, true},
		{"ascii", "Fiction", []string{"FICTION"}, true},
		{"a different word still does not match", "Fiction", []string{"Fictional"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules := shelfutil.NewNSFWRules([]shelfutil.NSFWFolder{{Path: tc.rule}})
			if got := rules.IsNSFWFolder(tc.below); got != tc.want {
				t.Errorf("rule %q against folder %v = %v, want %v", tc.rule, tc.below, got, tc.want)
			}
		})
	}
}
