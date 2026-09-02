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

// snapshotDir is a directory name no default covers: it carries no leading dot
// and is not one of the built-in names, so a shelf only skips it because its own
// shelf.json says so. Synology's snapshot directories are named like this.
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

// openConfiguredShelf builds a shelf holding the user's folder tree, a snapshot
// directory at every level and one "@eaDir", with the given shelf.json - or none
// when body is empty.
func openConfiguredShelf(t *testing.T, body string) *Shelf {
	t.Helper()

	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantExtraDirs(t, libRoot)
	if err := os.MkdirAll(path.Join(libRoot, booksFolder, "@eaDir", "nested"), 0755); err != nil {
		t.Fatalf("Failed to create @eaDir: %v", err)
	}
	writeFixtureBook(t, libRoot, "Fiction/Classics", "keep-0001")
	if body != "" {
		writeShelfConfig(t, libRoot, body)
	}

	return newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
}

func listedFolders(t *testing.T, s *Shelf) []string {
	t.Helper()

	folders, err := s.GetAllFolders()
	if err != nil {
		t.Fatalf("GetAllFolders: %v", err)
	}
	return folderStrings(folders)
}

// FolderPath.String() renders the root as "", which is how a listing reports a
// book sitting directly under books/.
var userFolders = []string{"", "Empty", "Fiction", "Fiction/Classics"}

func assertFolders(t *testing.T, got, want []string) {
	t.Helper()

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Expected layers %v, got %v", want, got)
	}
}

// The list in shelf.json is the shelf's rules, not an addition to them: a shelf
// that names its own directories skips those and only those. Naming "@Snapshot"
// therefore also stops "@eaDir" from being skipped, which is the cost of a
// single list and is pinned here rather than left to be discovered.
func TestShelfConfigIgnoredDirsReplacesTheDefaults(t *testing.T) {
	s := openConfiguredShelf(t, `{
  "schema_version": 1,
  "scan": { "ignored_dirs": [{ "name": "@Snapshot" }] }
}`)

	folders := listedFolders(t, s)
	for _, folder := range folders {
		if strings.Contains(folder, snapshotDir) {
			t.Errorf("Listed %q, want every %q layer skipped (all: %v)", folder, snapshotDir, folders)
		}
	}
	if !slices.Contains(folders, "@eaDir") {
		t.Errorf("Expected @eaDir to be listed once the shelf replaced the defaults, got %v", folders)
	}

	if got := scannedBookIDs(t, s); !slices.Equal(got, []string{"keep-0001"}) {
		t.Errorf("Expected only keep-0001 to be scanned, got %v", got)
	}
}

// A shelf that says nothing about scanning is read exactly as PlainShelf has
// always read it.
func TestShelfConfigWithoutTheKeyKeepsTheDefaults(t *testing.T) {
	for name, body := range map[string]string{
		"no file":          "",
		"no scan section":  `{"schema_version": 1}`,
		"empty scan":       `{"schema_version": 1, "scan": {}}`,
		"unrelated fields": `{"schema_version": 1, "reader": {"theme": "dark"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			folders := listedFolders(t, openConfiguredShelf(t, body))

			if slices.Contains(folders, "@eaDir") {
				t.Errorf("Expected @eaDir to stay skipped, got %v", folders)
			}
			if !slices.Contains(folders, snapshotDir) {
				t.Errorf("Expected %q to be listed without a rule for it, got %v", snapshotDir, folders)
			}
		})
	}
}

// An empty list is a shelf saying "skip nothing", which is not the same as
// saying nothing. Hidden directories are still skipped: that is a rule, not a
// name on the list.
func TestShelfConfigEmptyListSkipsOnlyHiddenDirectories(t *testing.T) {
	libRoot := t.TempDir()
	makeUserFolderTree(t, libRoot)
	plantIgnoredDirs(t, libRoot)
	writeShelfConfig(t, libRoot, `{"scan": {"ignored_dirs": []}}`)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	folders := listedFolders(t, s)
	for _, name := range []string{"@eaDir", "#recycle", "lost+found"} {
		if !slices.Contains(folders, name) {
			t.Errorf("Expected %q to be listed on a shelf that skips nothing, got %v", name, folders)
		}
	}
	for _, folder := range folders {
		for segment := range strings.SplitSeq(folder, "/") {
			if strings.HasPrefix(segment, ".") {
				t.Errorf("Listed hidden directory %q; the leading-dot rule is not configurable", folder)
			}
		}
	}
}

func TestShelfConfigIgnoredDirsAreCaseInsensitive(t *testing.T) {
	// The shelf may be read over SMB, where a share can report either spelling,
	// so a configured name folds case exactly like the defaults.
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": "@snapshot"}]}}`)

	for _, folder := range listedFolders(t, s) {
		if strings.Contains(strings.ToLower(folder), strings.ToLower(snapshotDir)) {
			t.Errorf("Expected %q to be skipped whatever its case, got layer %q", snapshotDir, folder)
		}
	}
}

// The object form is what lets a shelf say why it skips a directory this build
// has never heard of, and that sentence is what the user is shown later.
func TestShelfConfigEntryCarriesItsReason(t *testing.T) {
	const reason = "Synology snapshot directory"
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": "@Snapshot", "reason": "`+reason+`"}]}}`)

	if slices.Contains(listedFolders(t, s), snapshotDir) {
		t.Fatalf("Expected %q to be skipped", snapshotDir)
	}

	err := s.ValidateFolderPath(FolderPath{snapshotDir})
	var ignored *IgnoredFolderNameError
	if !errors.As(err, &ignored) {
		t.Fatalf("ValidateFolderPath(%q) = %v, want an IgnoredFolderNameError", snapshotDir, err)
	}
	if ignored.Reason != reason {
		t.Errorf("Reason = %q, want the shelf's own words %q", ignored.Reason, reason)
	}
}

// One unusable entry must not cost the rest of the file: the shelf opens, the
// entries that are usable apply, and the bad ones are reported to the log. An
// entry of the wrong type counts as one of those - the pCloud reader drops just
// that element, and the two must not read one file differently.
func TestShelfConfigSkipsUnusableEntries(t *testing.T) {
	// A bare name is one of the unusable shapes: an entry is always an object,
	// so that one list is not read two ways.
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": ""}, {"name": "with/separator"}, {"name": ".."}, 17, "@Snapshot", {"reason": "no name"}, {"name": "@Snapshot"}]}}`)

	// "@Snapshot" survived the bad entries; "@eaDir" and the directory under it
	// are ordinary folders, because this list replaced the defaults.
	want := append([]string{"", "@eaDir", "@eaDir/nested"}, userFolders[1:]...)
	assertFolders(t, listedFolders(t, s), want)
}

// A shelf whose settings cannot be read still opens, on the defaults: locking a
// user out of their library over a typo in an optional file is the worse
// failure.
func TestShelfConfigMalformedFallsBackToDefaults(t *testing.T) {
	for name, body := range map[string]string{
		"not json":            "this is not json",
		"wrong field type":    `{"scan": "everything"}`,
		"wrong list type":     `{"scan": {"ignored_dirs": "@Snapshot"}}`,
		"truncated mid-value": `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}`,
		// A Decoder stops at the end of the first value; the pCloud reader
		// parses the whole file and rejects what follows, so this one does too.
		"trailing object": `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}} {"scan": {}}`,
		"trailing debris": `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}} half an edit`,
	} {
		t.Run(name, func(t *testing.T) {
			folders := listedFolders(t, openConfiguredShelf(t, body))

			if !slices.Contains(folders, snapshotDir) || slices.Contains(folders, "@eaDir") {
				t.Errorf("Expected an unreadable %s to leave the defaults in place, got %v", shelfConfigFile, folders)
			}
		})
	}
}

// A file too large to be a settings file is skipped without being read, which is
// also what the pCloud client does with the size in its listing - it must not
// download megabytes onto a phone to discover the same thing.
func TestShelfConfigTooLargeIsSkipped(t *testing.T) {
	// Valid JSON that would otherwise apply, padded past the limit.
	padding := strings.Repeat(" ", maxShelfConfigBytes)
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}}`+padding)

	folders := listedFolders(t, s)
	if !slices.Contains(folders, snapshotDir) || slices.Contains(folders, "@eaDir") {
		t.Errorf("Expected an oversized %s to leave the defaults in place, got %v", shelfConfigFile, folders)
	}
}

// A shelf shared with a newer build carries a file this one only partly
// understands. Reading is all that happens to it, so the fields this build knows
// still apply rather than the whole file being discarded.
func TestShelfConfigNewerSchemaVersionStillApplies(t *testing.T) {
	s := openConfiguredShelf(t, `{
  "schema_version": 99,
  "scan": { "ignored_dirs": [{"name": "@Snapshot"}], "something_later": true },
  "reader": { "theme": "dark" }
}`)

	if slices.Contains(listedFolders(t, s), snapshotDir) {
		t.Errorf("Expected %q to be skipped, got the layer listed", snapshotDir)
	}
}

// A configured name is refused for the same reason "@eaDir" is on a shelf that
// kept the defaults: the directory would be created and then skipped by the very
// next scan.
func TestValidateFolderPathRejectsConfiguredNames(t *testing.T) {
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}}`)

	for _, folder := range []FolderPath{{snapshotDir}, {"Fiction", snapshotDir}, {"@snapshot"}} {
		err := s.ValidateFolderPath(folder)
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

	// The shelf replaced the defaults, so a name it no longer skips is a name it
	// can create - the setting decides, not the built-in list.
	if err := s.ValidateFolderPath(FolderPath{"@eaDir"}); err != nil {
		t.Errorf("Expected @eaDir to be creatable on a shelf that dropped it, got %v", err)
	}
}

func TestNewFolderRejectsConfiguredNames(t *testing.T) {
	libRoot := t.TempDir()
	writeShelfConfig(t, libRoot, `{"scan": {"ignored_dirs": [{"name": "@Snapshot", "reason": "kept by the NAS"}]}}`)
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	err := s.NewFolder(FolderPath{}, snapshotDir)
	if err == nil {
		t.Fatal("Expected NewFolder to reject a configured name, got nil")
	}
	// The message carries the shelf's own reason, since this is a rule the user
	// wrote and can take back.
	if !strings.Contains(err.Error(), "kept by the NAS") {
		t.Errorf("Expected the error to carry the shelf's reason, got %v", err)
	}

	if _, statErr := os.Stat(path.Join(libRoot, booksFolder, snapshotDir)); !os.IsNotExist(statErr) {
		t.Errorf("Expected no directory to be created, got %v", statErr)
	}
}

// The zero value is a shelf with no names at all, which is what an empty
// configured list produces; the hidden-directory rule survives it.
func TestIgnoreRulesZeroValue(t *testing.T) {
	var rules shelfutil.IgnoreRules

	if rules.IsIgnoredDir("@eaDir") {
		t.Error("Expected no names to be skipped by the zero rules")
	}
	if !rules.IsIgnoredDir(".git") {
		t.Error("Expected hidden directories to be skipped whatever the names are")
	}
	if got := rules.Names(); len(got) != 0 {
		t.Errorf("Names() = %v, want none", got)
	}
}
