package shelf

import (
	"errors"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

// ignoredDirFixtures is one directory of every ignored kind: the hidden
// helpers a sync client or a desktop OS leaves behind, plus the system
// directories NAS firmware and SMB shares create without a leading dot.
var ignoredDirFixtures = []string{
	"@eaDir",
	"#recycle",
	"$RECYCLE.BIN",
	"lost+found",
	".git",
	".stfolder",
	".dropbox.cache",
	".Spotlight-V100",
	".fseventsd",
	".TemporaryItems",
}

// userFolderDirs is the folder tree the user actually built, relative to books/.
var userFolderDirs = []string{
	"Fiction",
	"Fiction/Classics",
	"Empty",
}

func makeUserFolderTree(t *testing.T, libRoot string) {
	t.Helper()

	for _, dir := range userFolderDirs {
		if err := os.MkdirAll(path.Join(libRoot, booksFolder, dir), 0755); err != nil {
			t.Fatalf("Failed to create layer %s: %v", dir, err)
		}
	}
}

// plantIgnoredDirs drops one of every ignored directory under books/ and under
// every user folder, each holding a plain subdirectory. This is the Synology
// shape: "@eaDir" exists at every level, so a scanner that descends into it
// reports roughly twice the folders the user made.
func plantIgnoredDirs(t *testing.T, libRoot string) {
	t.Helper()

	parents := append([]string{"."}, userFolderDirs...)
	for _, parent := range parents {
		for _, name := range ignoredDirFixtures {
			noisePath := path.Join(libRoot, booksFolder, parent, name, "nested")
			if err := os.MkdirAll(noisePath, 0755); err != nil {
				t.Fatalf("Failed to create ignored dir %s: %v", noisePath, err)
			}
		}
	}
}

// writeFixtureBook creates a book package directly on disk, bypassing NewBook
// so that it can be planted inside a directory the API refuses to name.
func writeFixtureBook(t *testing.T, libRoot, folderDir, bookID string) {
	t.Helper()

	bookPath := path.Join(libRoot, booksFolder, folderDir, bookID+bookExtension)
	if err := os.MkdirAll(bookPath, 0755); err != nil {
		t.Fatalf("Failed to create book dir %s: %v", bookPath, err)
	}
	meta := `{"id":"` + bookID + `","title":"` + bookID + `"}`
	if err := os.WriteFile(path.Join(bookPath, "book.json"), []byte(meta), 0644); err != nil {
		t.Fatalf("Failed to write book.json in %s: %v", bookPath, err)
	}
}

func folderStrings(folders []FolderPath) []string {
	out := make([]string, 0, len(folders))
	for _, folder := range folders {
		out = append(out, folder.String())
	}
	return out
}

func TestGetAllFoldersIgnoresSystemDirectories(t *testing.T) {
	cleanRoot := t.TempDir()
	noisyRoot := t.TempDir()

	makeUserFolderTree(t, cleanRoot)
	makeUserFolderTree(t, noisyRoot)
	plantIgnoredDirs(t, noisyRoot)

	cleanShelf := newTestShelf(t, &ShelfConf{LibRoot: cleanRoot, LockMode: "none"})
	noisyShelf := newTestShelf(t, &ShelfConf{LibRoot: noisyRoot, LockMode: "none"})

	cleanFolders, err := cleanShelf.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders on clean shelf: %v", err)
	}
	noisyFolders, err := noisyShelf.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders on noisy shelf: %v", err)
	}

	clean := folderStrings(cleanFolders)
	noisy := folderStrings(noisyFolders)

	// Guard against the comparison passing because both sides are empty.
	found := slices.Contains(clean, "Fiction/Classics")
	if !found {
		t.Fatalf("Expected the clean shelf to report Fiction/Classics, got %v", clean)
	}

	if strings.Join(noisy, "|") != strings.Join(clean, "|") {
		t.Errorf("Expected the same layers with and without system directories.\n clean: %v\n noisy: %v", clean, noisy)
	}
}

func TestIterateBooksSkipsIgnoredDirectories(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)

	writeFixtureBook(t, libRoot, "Fiction/Classics", "keep-0001")
	// A deleted book still sitting in the NAS recycle bin must not come back.
	writeFixtureBook(t, libRoot, "#recycle", "recycled-0002")
	writeFixtureBook(t, libRoot, "Fiction/@eaDir", "sidecar-0003")
	writeFixtureBook(t, libRoot, ".stfolder", "synced-0004")

	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	var found []string
	if _, err := shelf.iterateShelfTree(nil, func(b *Book) bool {
		found = append(found, b.ID())
		return true
	}); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
	}
	slices.Sort(found)

	if len(found) != 1 || found[0] != "keep-0001" {
		t.Errorf("Expected only keep-0001 to be scanned, got %v", found)
	}
}

func TestBookCacheExportOmitsIgnoredDirectories(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantIgnoredDirs(t, libRoot)

	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})

	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)

	for _, folder := range cache.Folders {
		for segment := range strings.SplitSeq(folder, "/") {
			if segment != "" && shelf.ignore.IsIgnoredDir(segment) {
				t.Errorf("Exported cache layer %q contains ignored directory %q", folder, segment)
			}
		}
	}

	expected := []string{"/", "Empty", "Fiction", "Fiction/Classics"}
	if strings.Join(cache.Folders, "|") != strings.Join(expected, "|") {
		t.Errorf("Expected exported layers %v, got %v", expected, cache.Folders)
	}
}

// A shelf that has said nothing skips the built-in defaults, and each of them
// carries the reason the user is shown when one is refused as a folder name.
func TestDefaultIgnoreRules(t *testing.T) {
	rules := shelfutil.NewIgnoreRules(shelfutil.DefaultIgnoredDirs())

	tests := []struct {
		name    string
		ignored bool
	}{
		{"@eaDir", true},
		{"@eadir", true},
		{"#recycle", true},
		{"$RECYCLE.BIN", true},
		{"$Recycle.Bin", true},
		{"lost+found", true},
		{".git", true},
		{".stfolder", true},
		{".dropbox.cache", true},
		{".Spotlight-V100", true},
		{".fseventsd", true},
		{".TemporaryItems", true},
		{"Fiction", false},
		{"recycle", false},
		{"eaDir", false},
		{"lost found", false},
		{"my.folder", false},
	}

	for _, tt := range tests {
		reason, ignored := rules.MatchIgnoredDir(tt.name)
		if ignored != tt.ignored {
			t.Errorf("MatchIgnoredDir(%q) ignored = %v, want %v", tt.name, ignored, tt.ignored)
			continue
		}
		if ignored && reason == "" {
			t.Errorf("MatchIgnoredDir(%q) reported no reason; every default has one", tt.name)
		}
	}

	// Hidden directories are a rule rather than a listed name, so they are not
	// among the names and answer with the rule's own reason.
	if reason, _ := rules.MatchIgnoredDir(".git"); reason != shelfutil.DirIgnoreReasonHidden {
		t.Errorf("MatchIgnoredDir(.git) reason = %q, want the hidden-directory rule", reason)
	}
	if got := rules.Names(); !slices.Equal(got, []string{"#recycle", "$RECYCLE.BIN", "@eaDir", "lost+found"}) {
		t.Errorf("Names() = %v, want the four built-in names", got)
	}
}

// Which names are refused is the shelf's own rule now, so the check belongs to a
// shelf: a shelf with no configuration refuses exactly the defaults.
func TestValidateFolderPathRejectsIgnoredNames(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	for _, name := range ignoredDirFixtures {
		if err := s.ValidateFolderPath(FolderPath{name}); err == nil {
			t.Errorf("Expected ValidateFolderPath to reject %q, got nil", name)
		}
		if err := s.ValidateFolderPath(FolderPath{"Fiction", name}); err == nil {
			t.Errorf("Expected ValidateFolderPath to reject nested %q, got nil", name)
		}
	}

	if err := s.ValidateFolderPath(FolderPath{"Fiction", "Classics"}); err != nil {
		t.Errorf("Expected an ordinary layer to stay valid, got %v", err)
	}
}

// The API answers a name the scanners skip with its own explanation, so the
// rejection has to be distinguishable from every other invalid folder while
// still classifying as one, and it has to carry the reason out.
func TestValidateFolderPathReportsIgnoredNamesAsTheirOwnSentinel(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	for _, name := range ignoredDirFixtures {
		err := s.ValidateFolderPath(FolderPath{name})
		if !errors.Is(err, ErrIgnoredFolderName) {
			t.Errorf("ValidateFolderPath(%q) = %v, want ErrIgnoredFolderName", name, err)
		}
		if !errors.Is(err, ErrInvalidFolder) {
			t.Errorf("ValidateFolderPath(%q) = %v, want ErrInvalidFolder too", name, err)
		}

		var ignored *IgnoredFolderNameError
		if !errors.As(err, &ignored) {
			t.Errorf("ValidateFolderPath(%q) = %v, want an IgnoredFolderNameError", name, err)
			continue
		}
		if ignored.Folder != name || ignored.Reason == "" {
			t.Errorf("ValidateFolderPath(%q) reported %+v, want the name and a reason", name, ignored)
		}
	}

	// Any other rejection keeps the general sentinel alone.
	for _, name := range []string{"", "..", "with/separator", "Book.bookpkg"} {
		err := s.ValidateFolderPath(FolderPath{name})
		if !errors.Is(err, ErrInvalidFolder) {
			t.Errorf("ValidateFolderPath(%q) = %v, want ErrInvalidFolder", name, err)
		}
		if errors.Is(err, ErrIgnoredFolderName) {
			t.Errorf("ValidateFolderPath(%q) matched ErrIgnoredFolderName, want the general sentinel only", name)
		}
	}
}

func TestNewFolderRejectsIgnoredNames(t *testing.T) {
	shelf := newTestShelf(t, &ShelfConf{LibRoot: t.TempDir(), LockMode: "none"})

	err := shelf.NewFolder(FolderPath{}, "@eaDir")
	if err == nil {
		t.Fatal("Expected NewFolder to reject @eaDir, got nil")
	}
	// The message has to say why, since the directory would be creatable but
	// then invisible to every later scan.
	if !strings.Contains(err.Error(), "Synology index and thumbnail sidecar") {
		t.Errorf("Expected the error to carry the reason the name is skipped, got %v", err)
	}
}
