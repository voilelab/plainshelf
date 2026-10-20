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

const snapshotDir = "@Snapshot"

func writeShelfConfig(t *testing.T, libRoot, body string) {
	t.Helper()

	if err := os.WriteFile(path.Join(libRoot, shelfConfigFile), []byte(body), 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", shelfConfigFile, err)
	}
}

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
	if _, err := s.iterateShelfTree(nil, nil, func(b *Book) bool {
		found = append(found, b.ID())
		return true
	}); err != nil {
		t.Fatalf("iterateShelfTree: %v", err)
	}
	slices.Sort(found)
	return found
}

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

var userFolders = []string{"", "Empty", "Fiction", "Fiction/Classics"}

func assertFolders(t *testing.T, got, want []string) {
	t.Helper()

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Expected layers %v, got %v", want, got)
	}
}

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
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": "@snapshot"}]}}`)

	for _, folder := range listedFolders(t, s) {
		if strings.Contains(strings.ToLower(folder), strings.ToLower(snapshotDir)) {
			t.Errorf("Expected %q to be skipped whatever its case, got layer %q", snapshotDir, folder)
		}
	}
}

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

func TestShelfConfigSkipsUnusableEntries(t *testing.T) {
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": ""}, {"name": "with/separator"}, {"name": ".."}, 17, "@Snapshot", {"reason": "no name"}, {"name": "@Snapshot"}]}}`)

	want := append([]string{"", "@eaDir", "@eaDir/nested"}, userFolders[1:]...)
	assertFolders(t, listedFolders(t, s), want)
}

func TestShelfConfigMalformedFallsBackToDefaults(t *testing.T) {
	for name, body := range map[string]string{
		"not json":            "this is not json",
		"wrong field type":    `{"scan": "everything"}`,
		"wrong list type":     `{"scan": {"ignored_dirs": "@Snapshot"}}`,
		"truncated mid-value": `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}`,
		"trailing object":     `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}} {"scan": {}}`,
		"trailing debris":     `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}} half an edit`,
		"duplicate member":    `{"scan": {}, "scan": {"ignored_dirs": [{"name": "@Snapshot"}]}}`,
		"invalid utf-8":       "{\"scan\": {\"ignored_dirs\": [{\"name\": \"@Snapshot\", \"reason\": \"\xff\xfe\"}]}}",
	} {
		t.Run(name, func(t *testing.T) {
			folders := listedFolders(t, openConfiguredShelf(t, body))

			if !slices.Contains(folders, snapshotDir) || slices.Contains(folders, "@eaDir") {
				t.Errorf("Expected an unreadable %s to leave the defaults in place, got %v", shelfConfigFile, folders)
			}
		})
	}
}

func TestShelfConfigTooLargeIsSkipped(t *testing.T) {
	padding := strings.Repeat(" ", maxShelfConfigBytes)
	s := openConfiguredShelf(t, `{"scan": {"ignored_dirs": [{"name": "@Snapshot"}]}}`+padding)

	folders := listedFolders(t, s)
	if !slices.Contains(folders, snapshotDir) || slices.Contains(folders, "@eaDir") {
		t.Errorf("Expected an oversized %s to leave the defaults in place, got %v", shelfConfigFile, folders)
	}
}

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
	if !strings.Contains(err.Error(), "kept by the NAS") {
		t.Errorf("Expected the error to carry the shelf's reason, got %v", err)
	}

	if _, statErr := os.Stat(path.Join(libRoot, booksFolder, snapshotDir)); !os.IsNotExist(statErr) {
		t.Errorf("Expected no directory to be created, got %v", statErr)
	}
}

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
