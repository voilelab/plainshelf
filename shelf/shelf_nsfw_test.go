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

func TestNSFWBookFlagMarksOneBook(t *testing.T) {
	s := openNSFWShelf(t, "")

	assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)), []string{"flagged-0001"})
}

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

func TestNSFWTopLevelFolderMarksSubtree(t *testing.T) {
	s := openNSFWShelf(t, `{
		"content": {"nsfw_folders": [{"path": "Fiction"}]}
	}`)

	assertMarked(t, markedBookIDs(t, s, len(nsfwFixtureBooks)),
		[]string{"adult-0001", "adult-0002", "clean-0001", "denied-0001", "flagged-0001", "manga-0001"})
}

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

func TestNSFWAbsentConfigurationMarksNothing(t *testing.T) {
	libRoot := t.TempDir()
	writeNSFWFixtureBook(t, libRoot, "Fiction/Adult", "adult-0001", "")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	assertMarked(t, markedBookIDs(t, s, 1), nil)
}

func TestNSFWRulesZeroValue(t *testing.T) {
	var rules shelfutil.NSFWRules

	if rules.IsNSFWFolder(nil) || rules.IsNSFWFolder([]string{"Fiction"}) {
		t.Error("the zero value marks a folder; it must mark nothing")
	}
	if len(rules.Paths()) != 0 {
		t.Errorf("the zero value reports paths %v", rules.Paths())
	}
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal(%q): %v", value, err)
	}
	return string(encoded)
}

func TestNSFWExportCarriesAssembledAnswer(t *testing.T) {
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
		wantOwn := bookID == "flagged-0001"
		if entry.Meta.NSFW != wantOwn {
			t.Errorf("%s exported meta.nsfw = %v, want %v", bookID, entry.Meta.NSFW, wantOwn)
		}
	}
}

func TestNSFWUnmarkedShelfExportUnchanged(t *testing.T) {
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
