package shelf

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/jsonopt"
)

func newFolderMetaShelf(t *testing.T, folders ...FolderPath) (*Shelf, string) {
	t.Helper()

	libRoot := filepath.Join(t.TempDir(), "shelf")
	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})
	for _, folder := range folders {
		if err := s.NewFolder(folder[:len(folder)-1], folder[len(folder)-1]); err != nil {
			t.Fatalf("NewFolder(%v): %v", folder, err)
		}
	}
	return s, libRoot
}

func folderMetaFilePath(libRoot string, folder ...string) string {
	return filepath.Join(append([]string{libRoot, booksFolder}, append(folder, FolderMetaFile)...)...)
}

func writeFolderMeta(t *testing.T, libRoot string, content string, folder ...string) {
	t.Helper()
	if err := os.WriteFile(folderMetaFilePath(libRoot, folder...), []byte(content), 0o600); err != nil {
		t.Fatalf("write folder.json: %v", err)
	}
}

func readFolderMetaRaw(t *testing.T, libRoot string, folder ...string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(folderMetaFilePath(libRoot, folder...))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatalf("read folder.json: %v", err)
	}
	return string(data), true
}

func mustReadFolderMeta(t *testing.T, s *Shelf, folder FolderPath) FolderMeta {
	t.Helper()
	meta, err := s.ReadFolderMeta(folder)
	if err != nil {
		t.Fatalf("ReadFolderMeta(%v): %v", folder, err)
	}
	return meta
}

func TestFolderIconsAreSortedTablerNames(t *testing.T) {
	icons := FolderIcons()
	if !slices.IsSorted(icons) {
		t.Fatalf("folderIcons must stay sorted for IsFolderIcon: %v", icons)
	}
	if len(slices.Compact(slices.Clone(icons))) != len(icons) {
		t.Fatalf("folderIcons has duplicates: %v", icons)
	}
	name := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	for _, icon := range icons {
		if !name.MatchString(icon) || !IsFolderIcon(icon) {
			t.Errorf("icon %q is not a usable Tabler name", icon)
		}
	}
	if !IsFolderIcon("folder") || IsFolderIcon("") || IsFolderIcon("Folder") {
		t.Fatal("IsFolderIcon is not an exact-match lookup")
	}
}

func TestSetFolderIconRoundTripAndClear(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})

	if meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"}); meta != (FolderMeta{}) {
		t.Fatalf("a folder without folder.json read as %+v", meta)
	}

	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "rocket", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	raw, ok := readFolderMetaRaw(t, libRoot, "Fiction")
	want := "{\n  \"icon\": \"rocket\",\n  \"schema_version\": 1\n}"
	if !ok || raw != want {
		t.Fatalf("folder.json = %q, want %q", raw, want)
	}

	meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if meta.Icon != "rocket" || meta.DisplayIcon() != "rocket" || meta.ReadOnly || meta.Version == "" {
		t.Fatalf("ReadFolderMeta = %+v", meta)
	}

	// Clearing the only setting removes the file rather than leaving a stub.
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "", meta.Version); err != nil {
		t.Fatalf("SetFolderIcon(clear): %v", err)
	}
	if _, ok := readFolderMetaRaw(t, libRoot, "Fiction"); ok {
		t.Fatal("clearing the last setting left folder.json behind")
	}

	// Clearing with no file is a no-op, not an error.
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "", ""); err != nil {
		t.Fatalf("SetFolderIcon(clear, no file): %v", err)
	}

	// The folder listing is not affected by the file.
	if got := getFolderNames(t, s); !slices.Equal(got, []string{"", "Fiction"}) {
		t.Fatalf("folders = %v", got)
	}
}

func TestSetFolderIconKeepsUnknownMembers(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})
	writeFolderMeta(t, libRoot, `{"schema_version":1,"sort_key":-2.5,"future":{"b":[1,{"c":null}],"a":"x"},"icon":"book"}`, "Fiction")

	meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", meta.Version); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	assertFolderMetaMembers(t, libRoot, map[string]string{
		"schema_version": `1`,
		"sort_key":       `-2.5`,
		"future":         `{"b":[1,{"c":null}],"a":"x"}`,
		"icon":           `"star"`,
	})

	// Clearing removes the icon and nothing else, and keeps the file.
	meta = mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "", meta.Version); err != nil {
		t.Fatalf("SetFolderIcon(clear): %v", err)
	}
	assertFolderMetaMembers(t, libRoot, map[string]string{
		"schema_version": `1`,
		"sort_key":       `-2.5`,
		"future":         `{"b":[1,{"c":null}],"a":"x"}`,
	})
}

// An icon a newer build offers reads as the default here, and an edit of
// another member must carry it through rather than drop it as unknown.
func TestUnknownIconIsKeptByOtherWrites(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})
	writeFolderMeta(t, libRoot, `{"icon":"icon-from-a-newer-build"}`, "Fiction")

	meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if meta.Icon != "icon-from-a-newer-build" || meta.DisplayIcon() != "" || meta.ReadOnly {
		t.Fatalf("ReadFolderMeta = %+v", meta)
	}

	err := s.updateFolderMeta(FolderPath{"Fiction"}, meta.Version, func(obj jsonopt.Object) error {
		return obj.Set("sort_key", 3)
	})
	if err != nil {
		t.Fatalf("updateFolderMeta: %v", err)
	}
	assertFolderMetaMembers(t, libRoot, map[string]string{
		"schema_version": `1`,
		"icon":           `"icon-from-a-newer-build"`,
		"sort_key":       `3`,
	})
}

func assertFolderMetaMembers(t *testing.T, libRoot string, want map[string]string, folder ...string) {
	t.Helper()

	if len(folder) == 0 {
		folder = []string{"Fiction"}
	}
	raw, ok := readFolderMetaRaw(t, libRoot, folder...)
	if !ok {
		t.Fatal("folder.json is missing")
	}
	obj, err := jsonopt.ParseObject([]byte(raw))
	if err != nil {
		t.Fatalf("folder.json no longer parses: %v\n%s", err, raw)
	}
	if len(obj) != len(want) {
		t.Fatalf("folder.json members = %s, want %v", raw, want)
	}
	for name, value := range want {
		got, ok := obj[name]
		if !ok {
			t.Fatalf("folder.json lost %q:\n%s", name, raw)
		}
		compact := got.Clone()
		if err := compact.Compact(); err != nil {
			t.Fatalf("compact %q: %v", name, err)
		}
		if string(compact) != value {
			t.Fatalf("folder.json %q = %s, want %s", name, compact, value)
		}
	}
}

func TestReadFolderMetaFallsBackOnUnusableFiles(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		icon     string
		readOnly bool
	}{
		{name: "malformed", content: `{"icon":`, readOnly: true},
		{name: "array", content: `["rocket"]`, readOnly: true},
		{name: "null", content: `null`, readOnly: true},
		{name: "duplicate member", content: `{"icon":"book","icon":"star"}`, readOnly: true},
		{name: "newer schema", content: `{"schema_version":2,"icon":"book"}`, readOnly: true},
		{name: "zero schema", content: `{"schema_version":0,"icon":"book"}`, readOnly: true},
		{name: "schema of the wrong type", content: `{"schema_version":"1","icon":"book"}`, readOnly: true},
		{name: "oversized", content: `{"icon":"book","pad":"` + strings.Repeat("x", maxFolderMetaBytes) + `"}`, readOnly: true},
		{name: "icon of the wrong type", content: `{"icon":7}`},
		{name: "no schema version", content: `{"icon":"book"}`, icon: "book"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})
			writeFolderMeta(t, libRoot, tc.content, "Fiction")

			meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
			if meta.DisplayIcon() != tc.icon || meta.ReadOnly != tc.readOnly || (!tc.readOnly && meta.Version == "") {
				t.Fatalf("ReadFolderMeta = %+v, want icon %q, read-only %v", meta, tc.icon, tc.readOnly)
			}

			err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", meta.Version)
			if tc.readOnly {
				if !errors.Is(err, ErrFolderMetaNotWritable) {
					t.Fatalf("SetFolderIcon = %v, want ErrFolderMetaNotWritable", err)
				}
				if raw, _ := readFolderMetaRaw(t, libRoot, "Fiction"); raw != tc.content {
					t.Fatalf("a refused write changed folder.json to %q", raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetFolderIcon: %v", err)
			}
			if got := mustReadFolderMeta(t, s, FolderPath{"Fiction"}).Icon; got != "star" {
				t.Fatalf("icon after write = %q", got)
			}
		})
	}
}

func TestReadFolderMetaDoesNotWrite(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})
	content := `{ "icon" : "book" , "extra": [ 1 ] }`
	writeFolderMeta(t, libRoot, content, "Fiction")

	mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if raw, _ := readFolderMetaRaw(t, libRoot, "Fiction"); raw != content {
		t.Fatalf("reading rewrote folder.json to %q", raw)
	}
}

func TestReadFolderMetaWarnsOncePerProblem(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})
	warnings := countWarnings(s)

	writeFolderMeta(t, libRoot, `{"icon":`, "Fiction")
	for range 3 {
		mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	}
	if got := warnings.Load(); got != 1 {
		t.Fatalf("three reads of one broken file logged %d warnings, want 1", got)
	}

	// A different problem is news; so is the same one after a fix.
	writeFolderMeta(t, libRoot, `[]`, "Fiction")
	mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	writeFolderMeta(t, libRoot, `{}`, "Fiction")
	mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	writeFolderMeta(t, libRoot, `[]`, "Fiction")
	mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if got := warnings.Load(); got != 3 {
		t.Fatalf("warnings = %d, want 3", got)
	}
}

// countWarnings replaces the shelf's log handler with one that counts
// warnings, after the initial scan so its own logging is not counted.
func countWarnings(s *Shelf) *atomic.Int64 {
	counter := &warnCounter{}
	s.Logger.Logger = slog.New(counter)
	return &counter.n
}

type warnCounter struct{ n atomic.Int64 }

func (c *warnCounter) Enabled(context.Context, slog.Level) bool { return true }
func (c *warnCounter) Handle(_ context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn {
		c.n.Add(1)
	}
	return nil
}
func (c *warnCounter) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *warnCounter) WithGroup(string) slog.Handler      { return c }

func TestSetFolderIconRefusals(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})

	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "not-an-icon", ""); !errors.Is(err, ErrInvalidFolderIcon) {
		t.Fatalf("unknown icon: %v, want ErrInvalidFolderIcon", err)
	}
	if err := s.SetFolderIcon(FolderPath{}, "book", ""); !errors.Is(err, ErrInvalidFolder) {
		t.Fatalf("root: %v, want ErrInvalidFolder", err)
	}
	if _, err := os.Stat(filepath.Join(libRoot, booksFolder, FolderMetaFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused root write left books/folder.json: %v", err)
	}
	if err := s.SetFolderIcon(FolderPath{"Missing"}, "book", ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing folder: %v, want ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Join(libRoot, booksFolder, "Missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a write to a missing folder created it")
	}
	if meta := mustReadFolderMeta(t, s, FolderPath{}); meta != (FolderMeta{}) {
		t.Fatalf("the root read as %+v", meta)
	}
}

func TestSetFolderIconRefusesAStaleVersion(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})

	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "book", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}
	read := mustReadFolderMeta(t, s, FolderPath{"Fiction"})

	// Edited by hand after the caller's read.
	edited := `{"icon":"moon","schema_version":1}`
	writeFolderMeta(t, libRoot, edited, "Fiction")

	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", read.Version); !errors.Is(err, ErrFolderMetaConflict) {
		t.Fatalf("stale write: %v, want ErrFolderMetaConflict", err)
	}
	if raw, _ := readFolderMetaRaw(t, libRoot, "Fiction"); raw != edited {
		t.Fatalf("a refused write changed folder.json to %q", raw)
	}

	// A caller that read nothing must not create over a file that appeared.
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", ""); !errors.Is(err, ErrFolderMetaConflict) {
		t.Fatalf("write expecting no file: %v, want ErrFolderMetaConflict", err)
	}

	fresh := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", fresh.Version); err != nil {
		t.Fatalf("write with a fresh version: %v", err)
	}
}

// failingRenameFS makes every atomic write fail at its last step.
type failingRenameFS struct{ fsutil.FS }

func (f failingRenameFS) Rename(string, string) error { return errors.New("rename refused") }

func TestFailedFolderMetaWriteKeepsTheOriginal(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"})
	original := `{"icon":"book","keep":true}`
	writeFolderMeta(t, libRoot, original, "Fiction")
	meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})

	s.dbRoot = failingRenameFS{writeRootForTest(t, s)}
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", meta.Version); err == nil {
		t.Fatal("SetFolderIcon reported success through a failing rename")
	}
	if raw, _ := readFolderMetaRaw(t, libRoot, "Fiction"); raw != original {
		t.Fatalf("a failed write changed folder.json to %q", raw)
	}
	entries, err := os.ReadDir(filepath.Join(libRoot, booksFolder, "Fiction"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("a failed write left %d entries beside folder.json", len(entries)-1)
	}
}

func TestReadOnlyShelfReadsFolderMetaAndRefusesWrites(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "shelf")
	if err := os.MkdirAll(filepath.Join(libRoot, booksFolder, "Fiction"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"icon":"book"}`
	writeFolderMeta(t, libRoot, content, "Fiction")

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, ReadOnly: true})
	meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if meta.Icon != "book" {
		t.Fatalf("ReadFolderMeta = %+v", meta)
	}
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "star", meta.Version); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("SetFolderIcon on a read-only shelf: %v, want ErrReadOnly", err)
	}
	if raw, _ := readFolderMetaRaw(t, libRoot, "Fiction"); raw != content {
		t.Fatalf("folder.json changed to %q", raw)
	}
}

func TestFolderMetaFileNameIsReserved(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"}, FolderPath{"Other"})

	for _, name := range []string{FolderMetaFile, "Folder.JSON"} {
		if err := s.NewFolder(FolderPath{"Fiction"}, name); !errors.Is(err, ErrReservedFolderName) || !errors.Is(err, ErrInvalidFolder) {
			t.Errorf("NewFolder(%q) = %v, want ErrReservedFolderName", name, err)
		}
		if err := s.RenameFolder(FolderPath{"Other"}, name); !errors.Is(err, ErrReservedFolderName) {
			t.Errorf("RenameFolder(Other, %q) = %v, want ErrReservedFolderName", name, err)
		}
	}

	// A folder of that name made outside PlainShelf stays listed, its parent's
	// metadata reads as none and refuses writes, and it can be renamed away.
	if err := os.Mkdir(folderMetaFilePath(libRoot, "Fiction"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if _, err := s.RescanUnthrottled(); err != nil {
		t.Fatalf("RescanUnthrottled: %v", err)
	}
	if got := getFolderNames(t, s); !slices.Contains(got, "Fiction/"+FolderMetaFile) {
		t.Fatalf("folders = %v, want the existing folder.json directory listed", got)
	}
	meta := mustReadFolderMeta(t, s, FolderPath{"Fiction"})
	if !meta.ReadOnly || meta.DisplayIcon() != "" {
		t.Fatalf("parent of a folder.json directory read as %+v", meta)
	}
	// The listing says the same, so a client does not offer a write that fails.
	if listed, ok := mustListFolderMeta(t, s)["Fiction"]; !ok || !listed.ReadOnly {
		t.Fatalf("ListFolderMeta reported the parent of a folder.json directory as %+v (listed %v)", listed, ok)
	}
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "book", meta.Version); !errors.Is(err, ErrFolderMetaNotWritable) {
		t.Fatalf("SetFolderIcon over a folder.json directory: %v, want ErrFolderMetaNotWritable", err)
	}
	if err := s.MoveFolder(FolderPath{"Fiction", FolderMetaFile}, FolderPath{"Other"}); !errors.Is(err, ErrReservedFolderName) {
		t.Fatalf("MoveFolder of a folder.json directory: %v, want ErrReservedFolderName", err)
	}
	if err := s.RenameFolder(FolderPath{"Fiction", FolderMetaFile}, "Renamed"); err != nil {
		t.Fatalf("renaming the folder.json directory away: %v", err)
	}
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "book", ""); err != nil {
		t.Fatalf("SetFolderIcon after the rename: %v", err)
	}
}

func TestDeleteFolderHoldingOnlyItsMetadata(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Empty"}, FolderPath{"Stray"}, FolderPath{"Parent"}, FolderPath{"Parent", "Child"}, FolderPath{"Carried"})
	for _, folder := range []FolderPath{{"Empty"}, {"Stray"}, {"Parent"}} {
		if err := s.SetFolderIcon(folder, "book", ""); err != nil {
			t.Fatalf("SetFolderIcon(%v): %v", folder, err)
		}
	}
	if err := os.WriteFile(filepath.Join(libRoot, booksFolder, "Stray", "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// PruneFolder is the strict form: metadata is content to it.
	if err := s.PruneFolder(FolderPath{"Empty"}, ""); err == nil {
		t.Fatal("PruneFolder deleted a folder holding folder.json")
	}
	if _, ok := readFolderMetaRaw(t, libRoot, "Empty"); !ok {
		t.Fatal("a refused prune removed folder.json")
	}
	if err := s.PruneFolder(FolderPath{"Empty"}, folderMetaVersion([]byte("{}"))); err == nil {
		t.Fatal("PruneFolder deleted a folder holding a folder.json it was not told was carried")
	}
	if err := s.PruneFolder(FolderPath{"Parent"}, ""); err == nil {
		t.Fatal("PruneFolder deleted a folder holding a subfolder")
	}

	if err := s.SetFolderIcon(FolderPath{"Carried"}, "moon", ""); err != nil {
		t.Fatalf("SetFolderIcon(Carried): %v", err)
	}
	carried, err := s.ReadFolderMeta(FolderPath{"Carried"})
	if err != nil {
		t.Fatalf("ReadFolderMeta(Carried): %v", err)
	}
	if err := s.PruneFolder(FolderPath{"Carried"}, carried.Version); err != nil {
		t.Fatalf("PruneFolder with the carried version: %v", err)
	}

	if err := s.DeleteFolder(FolderPath{"Empty"}); err != nil {
		t.Fatalf("DeleteFolder(Empty): %v", err)
	}
	if _, err := os.Stat(filepath.Join(libRoot, booksFolder, "Empty")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Empty survived: %v", err)
	}
	for _, folder := range []FolderPath{{"Stray"}, {"Parent"}} {
		if err := s.DeleteFolder(folder); err == nil {
			t.Errorf("DeleteFolder(%v) deleted a folder with other content", folder)
		}
		if _, ok := readFolderMetaRaw(t, libRoot, folder...); !ok {
			t.Errorf("a refused delete of %v removed its folder.json", folder)
		}
	}
	if err := s.DeleteFolder(FolderPath{}); err == nil {
		t.Fatal("DeleteFolder deleted the root")
	}
}

func TestFolderMetaMovesWithItsFolder(t *testing.T) {
	s, libRoot := newFolderMetaShelf(t, FolderPath{"Fiction"}, FolderPath{"Archive"})
	if err := s.SetFolderIcon(FolderPath{"Fiction"}, "rocket", ""); err != nil {
		t.Fatalf("SetFolderIcon: %v", err)
	}

	if err := s.RenameFolder(FolderPath{"Fiction"}, "SF"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if got := mustReadFolderMeta(t, s, FolderPath{"SF"}).Icon; got != "rocket" {
		t.Fatalf("icon after rename = %q", got)
	}
	if err := s.MoveFolder(FolderPath{"SF"}, FolderPath{"Archive"}); err != nil {
		t.Fatalf("MoveFolder: %v", err)
	}
	if got := mustReadFolderMeta(t, s, FolderPath{"Archive", "SF"}).Icon; got != "rocket" {
		t.Fatalf("icon after move = %q", got)
	}

	// A move made outside PlainShelf carries the file the same way.
	if err := os.Rename(filepath.Join(libRoot, booksFolder, "Archive", "SF"), filepath.Join(libRoot, booksFolder, "Moved")); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if got := mustReadFolderMeta(t, s, FolderPath{"Moved"}).Icon; got != "rocket" {
		t.Fatalf("icon after an outside move = %q", got)
	}
	if _, ok := readFolderMetaRaw(t, libRoot, path.Join("Archive", "SF")); ok {
		t.Fatal("folder.json stayed behind at the old path")
	}
}
