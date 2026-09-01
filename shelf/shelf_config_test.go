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

// snapshotDir is a directory name no built-in rule covers: it carries no leading
// dot and is not on the system list, so a shelf only skips it because its own
// shelf.json says so. Synology's own snapshot directories are named like this.
const snapshotDir = "@Snapshot"

func writeShelfConfig(t *testing.T, libRoot, body string) {
	t.Helper()

	if err := os.WriteFile(path.Join(libRoot, shelfConfigFile), []byte(body), 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", shelfConfigFile, err)
	}
}

// plantExtraDirs drops one snapshotDir under books/ and under every user folder,
// each holding a book. This is the shape that makes the setting worth having:
// like "@eaDir", the directory appears at every level, so a scanner that treats
// it as a folder roughly doubles the tree the user sees.
func plantExtraDirs(t *testing.T, libRoot string) {
	t.Helper()

	parents := append([]string{"."}, userFolderDirs...)
	for i, parent := range parents {
		dir := path.Join(parent, snapshotDir)
		if err := os.MkdirAll(path.Join(libRoot, booksFolder, dir, "nested"), 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", dir, err)
		}
		writeFixtureBook(t, libRoot, dir, "snapshot-000"+string(rune('1'+i)))
	}
}

func scannedBookIDs(t *testing.T, s *Shelf) []string {
	t.Helper()

	var found []string
	if _, err := s.iterateShelfTree(nil, func(b *Book) bool {
		found = append(found, b.ID())
		return true
	}); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
	}
	slices.Sort(found)
	return found
}

// The fixture is only meaningful if the same tree without the setting does show
// the directory, so the two halves are checked against each other rather than
// against a hard-coded list that could pass for the wrong reason.
func TestShelfConfigExtraIgnoredDirsHidesFoldersAndBooks(t *testing.T) {
	configured := t.TempDir()
	bare := t.TempDir()

	for _, libRoot := range []string{configured, bare} {
		makeUserFolderTree(t, libRoot)
		plantExtraDirs(t, libRoot)
		writeFixtureBook(t, libRoot, "Fiction/Classics", "keep-0001")
	}
	writeShelfConfig(t, configured, `{
  "schema_version": 1,
  "scan": { "extra_ignored_dirs": ["@Snapshot"] }
}`)

	configuredShelf := newTestShelf(t, &ShelfConf{LibRoot: configured, LockMode: "none"})
	bareShelf := newTestShelf(t, &ShelfConf{LibRoot: bare, LockMode: "none"})

	bareFolders, err := bareShelf.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders on the unconfigured shelf: %v", err)
	}
	if !slices.Contains(folderStrings(bareFolders), snapshotDir) {
		t.Fatalf("Expected the unconfigured shelf to still report %q, got %v", snapshotDir, folderStrings(bareFolders))
	}

	folders, err := configuredShelf.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders on the configured shelf: %v", err)
	}
	for _, folder := range folderStrings(folders) {
		if strings.Contains(folder, snapshotDir) {
			t.Errorf("Configured shelf reported %q, want every %q layer skipped (all: %v)", folder, snapshotDir, folderStrings(folders))
		}
	}

	// The user's own folders are untouched, including the empty one, which has
	// no book to rebuild it from.
	// FolderPath.String() renders the root as "", which is how a listing reports
	// a book that sits directly under books/.
	expected := []string{"", "Empty", "Fiction", "Fiction/Classics"}
	if got := folderStrings(folders); strings.Join(got, "|") != strings.Join(expected, "|") {
		t.Errorf("Expected layers %v, got %v", expected, got)
	}

	if got := scannedBookIDs(t, configuredShelf); len(got) != 1 || got[0] != "keep-0001" {
		t.Errorf("Expected only keep-0001 to be scanned, got %v", got)
	}
}

func TestShelfConfigExtraIgnoredDirsAreCaseInsensitive(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantExtraDirs(t, libRoot)
	// The shelf may be read over SMB, where a share can report either spelling,
	// so the setting folds case exactly like the built-in list.
	writeShelfConfig(t, libRoot, `{"scan": {"extra_ignored_dirs": ["@snapshot"]}}`)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	folders, err := s.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	for _, folder := range folderStrings(folders) {
		if strings.Contains(strings.ToLower(folder), strings.ToLower(snapshotDir)) {
			t.Errorf("Expected %q to be skipped whatever its case, got layer %q", snapshotDir, folder)
		}
	}
}

// The setting can only add. A shelf.json that names a built-in directory is
// redundant rather than an unignore, and nothing it can say brings "@eaDir"
// back as a folder.
func TestShelfConfigCannotUnignoreBuiltins(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantIgnoredDirs(t, libRoot)
	writeFixtureBook(t, libRoot, "@eaDir", "sidecar-0001")
	writeFixtureBook(t, libRoot, "Fiction/Classics", "keep-0001")
	writeShelfConfig(t, libRoot, `{"scan": {"extra_ignored_dirs": ["@eaDir", ".git"]}}`)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	folders, err := s.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	// FolderPath.String() renders the root as "", which is how a listing reports
	// a book that sits directly under books/.
	expected := []string{"", "Empty", "Fiction", "Fiction/Classics"}
	if got := folderStrings(folders); strings.Join(got, "|") != strings.Join(expected, "|") {
		t.Errorf("Expected layers %v, got %v", expected, got)
	}
	if got := scannedBookIDs(t, s); len(got) != 1 || got[0] != "keep-0001" {
		t.Errorf("Expected only keep-0001 to be scanned, got %v", got)
	}
}

// One unusable entry must not cost the rest of the file: the shelf opens, the
// entries that are usable apply, and the bad ones are reported to the log.
func TestShelfConfigSkipsUnusableEntries(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantExtraDirs(t, libRoot)
	writeShelfConfig(t, libRoot, `{"scan": {"extra_ignored_dirs": ["", "with/separator", "..", "@Snapshot"]}}`)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	folders, err := s.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	// FolderPath.String() renders the root as "", which is how a listing reports
	// a book that sits directly under books/.
	expected := []string{"", "Empty", "Fiction", "Fiction/Classics"}
	if got := folderStrings(folders); strings.Join(got, "|") != strings.Join(expected, "|") {
		t.Errorf("Expected layers %v, got %v", expected, got)
	}
}

// A shelf whose settings cannot be read still opens: locking a user out of their
// library over a typo in an optional file is the worse failure.
func TestShelfConfigMalformedFallsBackToBuiltins(t *testing.T) {
	for name, body := range map[string]string{
		"not json":            "this is not json",
		"wrong element type":  `{"scan": {"extra_ignored_dirs": [17]}}`,
		"wrong field type":    `{"scan": "everything"}`,
		"truncated mid-value": `{"scan": {"extra_ignored_dirs": ["@Snapshot"`,
	} {
		t.Run(name, func(t *testing.T) {
			libRoot := t.TempDir()
			makeUserFolderTree(t, libRoot)
			plantExtraDirs(t, libRoot)
			writeShelfConfig(t, libRoot, body)

			s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

			folders, err := s.GetAllFolders()
			if err != nil {
				t.Fatalf("GetAllFolders: %v", err)
			}
			if !slices.Contains(folderStrings(folders), snapshotDir) {
				t.Errorf("Expected an unreadable %s to leave the built-in rules in place, got %v", shelfConfigFile, folderStrings(folders))
			}
		})
	}
}

// A shelf shared with a newer build carries a file this one only partly
// understands. Reading is all that happens to it, so the fields this build knows
// still apply rather than the whole file being discarded.
func TestShelfConfigNewerSchemaVersionStillApplies(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantExtraDirs(t, libRoot)
	writeShelfConfig(t, libRoot, `{
  "schema_version": 99,
  "scan": { "extra_ignored_dirs": ["@Snapshot"], "something_later": true },
  "reader": { "theme": "dark" }
}`)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	folders, err := s.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	if slices.Contains(folderStrings(folders), snapshotDir) {
		t.Errorf("Expected %q to be skipped, got %v", snapshotDir, folderStrings(folders))
	}
}

// A configured name is refused for the same reason "@eaDir" is: the directory
// would be created and then skipped by the very next scan.
func TestValidateFolderPathRejectsConfiguredNames(t *testing.T) {
	libRoot := t.TempDir()
	writeShelfConfig(t, libRoot, `{"scan": {"extra_ignored_dirs": ["@Snapshot"]}}`)
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	for _, folder := range []FolderPath{{snapshotDir}, {"Fiction", snapshotDir}, {"@snapshot"}} {
		err := s.ValidateFolderPath(folder)
		// The API tells a name the user ignored themselves apart from a system
		// name they cannot change, so the rejection carries its own sentinel as
		// well as the general ones.
		if !errors.Is(err, ErrConfiguredIgnoredFolderName) {
			t.Errorf("ValidateFolderPath(%v) = %v, want ErrConfiguredIgnoredFolderName", folder, err)
		}
		if !errors.Is(err, ErrIgnoredFolderName) {
			t.Errorf("ValidateFolderPath(%v) = %v, want ErrIgnoredFolderName", folder, err)
		}
		if !errors.Is(err, ErrInvalidFolder) {
			t.Errorf("ValidateFolderPath(%v) = %v, want ErrInvalidFolder too", folder, err)
		}
	}

	if err := s.ValidateFolderPath(FolderPath{"Fiction", "Classics"}); err != nil {
		t.Errorf("Expected an ordinary layer to stay valid, got %v", err)
	}

	// A built-in name keeps the general sentinel alone: it is not something the
	// user can take back by editing their settings.
	if err := s.ValidateFolderPath(FolderPath{"@eaDir"}); errors.Is(err, ErrConfiguredIgnoredFolderName) {
		t.Errorf("ValidateFolderPath(@eaDir) = %v, want the built-in rejection, not the configured one", err)
	}

	// The built-in floor is a property of the name alone, so it stays answerable
	// without a shelf; only the configured names need one.
	if err := validateFolderPath(FolderPath{snapshotDir}); err != nil {
		t.Errorf("Expected the built-in rules to accept %q, got %v", snapshotDir, err)
	}
}

func TestNewFolderRejectsConfiguredNames(t *testing.T) {
	libRoot := t.TempDir()
	writeShelfConfig(t, libRoot, `{"scan": {"extra_ignored_dirs": ["@Snapshot"]}}`)
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	err := s.NewFolder(FolderPath{}, snapshotDir)
	if err == nil {
		t.Fatal("Expected NewFolder to reject a configured name, got nil")
	}
	// The message names the file, because unlike "@eaDir" this is a rule the
	// user wrote and can take back.
	if !strings.Contains(err.Error(), shelfConfigFile) {
		t.Errorf("Expected the error to name %s, got %v", shelfConfigFile, err)
	}

	if _, statErr := os.Stat(path.Join(libRoot, booksFolder, snapshotDir)); !os.IsNotExist(statErr) {
		t.Errorf("Expected no directory to be created, got %v", statErr)
	}
}

func TestIgnoreRules(t *testing.T) {
	rules := shelfutil.NewIgnoreRules([]string{"@Snapshot", "Thumbs"})

	tests := []struct {
		name    string
		ignored bool
		extra   bool
	}{
		{"@Snapshot", true, true},
		{"@snapshot", true, true},
		{"THUMBS", true, true},
		{"@eaDir", true, false},
		{".git", true, false},
		{"Fiction", false, false},
	}
	for _, tt := range tests {
		if got := rules.IsIgnoredDir(tt.name); got != tt.ignored {
			t.Errorf("IsIgnoredDir(%q) = %v, want %v", tt.name, got, tt.ignored)
		}
		if got := rules.IsExtraIgnoredDir(tt.name); got != tt.extra {
			t.Errorf("IsExtraIgnoredDir(%q) = %v, want %v", tt.name, got, tt.extra)
		}
	}

	// The zero value is the built-in list alone, which is what a shelf with no
	// configuration - and every test shelf that predates this file - holds.
	var none shelfutil.IgnoreRules
	if none.IsIgnoredDir("@Snapshot") {
		t.Error("Expected the zero rules to skip nothing beyond the built-in list")
	}
	if !none.IsIgnoredDir("@eaDir") {
		t.Error("Expected the zero rules to still skip @eaDir")
	}
}
