package shelf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// readOnlyFS exposes only the read half of a filesystem, standing in for a
// shelf this build cannot write: a read-only mount, or a reader that was handed
// nothing else.
type readOnlyFS struct {
	fsutil.ReadFS
}

// A book opened on a read-only filesystem still reads. Every mutation is
// refused with fsutil.ErrReadOnly — before touching anything — instead of
// panicking or writing.
//
// The fixture is the committed testdata tree on purpose: a guard that let a
// write through would show up as a dirty working tree, not just a red test.
func TestBookOnReadOnlyFSRefusesWrites(t *testing.T) {
	root := readOnlyFS{testdataFS(t)}

	book, err := openBook(root, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("openBook: %v", err)
	}
	if book.Title() == "" {
		t.Fatal("expected a read-only book to still read its metadata")
	}
	if _, _, err := book.OpenCover(); err != nil {
		t.Fatalf("OpenCover: %v", err)
	}

	source, err := book.GetSource("20260315-a1")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}

	writes := map[string]func() error{
		"SetMeta":           func() error { return book.SetMeta(book.GetMeta()) },
		"SetCover":          func() error { return book.SetCover([]byte("png data"), ".png") },
		"DeleteCover":       func() error { return book.DeleteCover() },
		"SetCurrentSource":  func() error { return book.SetCurrentSource("20260315-a1") },
		"NewSource":         func() error { _, err := book.NewSource(nil); return err },
		"DeleteSource":      func() error { return book.DeleteSource("20260315-a1") },
		"UpdateContent":     func() error { return source.UpdateContent(strings.NewReader("new text")) },
		"UpdateComment":     func() error { return source.UpdateComment("comment") },
		"UpdateHash":        func() error { return source.UpdateHash() },
		"WriteAsset":        func() error { return source.WriteAsset("img-0001.png", []byte("png data")) },
		"DeleteAsset":       func() error { return source.DeleteAsset("img-0001.png") },
		"UpgradeLegacyToV1": func() error { return source.UpgradeLegacyToSchemaV1(BookFormatMarkdown, nil) },
	}

	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			if err := write(); !errors.Is(err, fsutil.ErrReadOnly) {
				t.Errorf("%s error = %v, want %v", name, err, fsutil.ErrReadOnly)
			}
		})
	}
}

// A refused write must not leave the in-memory source ahead of meta.json. Each
// of these mutates r.meta on its way to writebackMeta, so narrowing the
// filesystem at the write itself would report ErrReadOnly while GetMeta already
// reported the value that never reached disk.
func TestRefusedSourceWriteLeavesMetaUntouched(t *testing.T) {
	root := readOnlyFS{testdataFS(t)}

	book, err := openBook(root, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("openBook: %v", err)
	}
	source, err := book.GetSource("20260315-a1")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	before := source.GetMeta()

	writes := map[string]func() error{
		"UpdateComment":     func() error { return source.UpdateComment("comment that cannot be stored") },
		"UpdateHash":        func() error { return source.UpdateHash() },
		"UpgradeLegacyToV1": func() error { return source.UpgradeLegacyToSchemaV1(BookFormatMarkdown, nil) },
	}

	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			if err := write(); !errors.Is(err, fsutil.ErrReadOnly) {
				t.Fatalf("%s error = %v, want %v", name, err, fsutil.ErrReadOnly)
			}

			after := source.GetMeta()
			if after.Comment != before.Comment {
				t.Errorf("comment = %q, want the unpersisted value to be discarded (%q)", after.Comment, before.Comment)
			}
			if after.MD5Hash != before.MD5Hash {
				t.Errorf("md5 hash = %q, want %q", after.MD5Hash, before.MD5Hash)
			}
			if after.SchemaVersion != before.SchemaVersion {
				t.Errorf("schema version = %d, want %d", after.SchemaVersion, before.SchemaVersion)
			}
			if after.Format != before.Format {
				t.Errorf("format = %q, want %q", after.Format, before.Format)
			}
		})
	}
}

// seedReadOnlyShelf builds a complete shelf - a book with metadata, a cover, a
// source and an asset - and closes it, so what follows opens a shelf that a
// user could plausibly have copied onto a read-only medium.
func seedReadOnlyShelf(t *testing.T, libRoot string) string {
	t.Helper()

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	book, err := s.NewBook(Layers{"fiction"}, "Read Only Book")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := book.SetCover([]byte("cover bytes"), ".png"); err != nil {
		t.Fatalf("SetCover: %v", err)
	}
	source, err := book.NewSource(strings.NewReader(readOnlyContent))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := source.WriteAsset("img-0001.png", []byte("asset bytes")); err != nil {
		t.Fatalf("WriteAsset: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	return book.ID()
}

const readOnlyContent = "# Chapter 1\n\nText that must still be readable.\n"

// treeSnapshot fingerprints every path under root, so that "this shelf was not
// written to" can be asserted on the shelf itself rather than on the operations
// that were refused.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(pth string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, pth)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		snapshot[rel] = fmt.Sprintf("dir=%t size=%d mtime=%d", entry.IsDir(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	return snapshot
}

// denyWrites takes the write bit off the whole tree, which is what a read-only
// mount or a restored backup looks like from the process's side.
//
// It is a second line of defense rather than the assertion: a test running as
// root is not stopped by a mode bit, so what actually proves the shelf was left
// alone is the tree snapshot around it.
func denyWrites(t *testing.T, root string) {
	t.Helper()

	chmodTree(t, root, 0o555, 0o444)
	// Restored before the temp directory is removed, which is registered
	// earlier and therefore runs after this.
	t.Cleanup(func() { chmodTree(t, root, 0o755, 0o644) })
}

func chmodTree(t *testing.T, root string, dirMode, fileMode fs.FileMode) {
	t.Helper()

	// Top-down is safe in both directions: 0555 still lets the walk descend,
	// and restoring 0755 only ever widens what it can reach.
	err := filepath.WalkDir(root, func(pth string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := fileMode
		if entry.IsDir() {
			mode = dirMode
		}
		return os.Chmod(pth, mode)
	})
	if err != nil {
		t.Fatalf("chmod %q: %v", root, err)
	}
}

// A read-only shelf opens, serves every read the reader needs, and leaves the
// shelf byte for byte as it found it - including the runtime files under app/,
// which a writable shelf refreshes on the way in and out.
//
// The write-enabling options are set on purpose: read_only has to override
// them, and if it did not, the lock file and the exported book cache would both
// show up in the snapshot comparison below.
func TestReadOnlyShelfReadsWithoutWriting(t *testing.T) {
	libRoot := t.TempDir()
	bookID := seedReadOnlyShelf(t, libRoot)

	before := treeSnapshot(t, libRoot)
	denyWrites(t, libRoot)

	s, err := NewShelf(&ShelfConf{
		LibRoot:           libRoot,
		ReadOnly:          true,
		LockMode:          "flock",
		BookCacheWriterID: "readonly-test",
		BookCacheInterval: "0s",
	})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			s.Close()
		}
	})
	if err := s.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	if !s.ReadOnly() {
		t.Error("ReadOnly() = false, want true on a shelf opened read-only")
	}

	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if len(books) != 1 || books[0].ID() != bookID {
		t.Fatalf("ListBooks returned %d books, want the seeded %q", len(books), bookID)
	}

	book, err := s.GetBook(bookID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if book.Title() != "Read Only Book" {
		t.Errorf("Title() = %q, want %q", book.Title(), "Read Only Book")
	}
	if !book.Layers().Equal(Layers{"fiction"}) {
		t.Errorf("Layers() = %v, want [fiction]", book.Layers())
	}

	cover, ext, err := book.OpenCover()
	if err != nil {
		t.Fatalf("OpenCover: %v", err)
	}
	if string(cover) != "cover bytes" || ext != ".png" {
		t.Errorf("OpenCover() = %q, %q, want %q, %q", cover, ext, "cover bytes", ".png")
	}

	source, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	content, err := source.Open()
	if err != nil {
		t.Fatalf("Source.Open: %v", err)
	}
	raw, err := io.ReadAll(content)
	content.Close()
	if err != nil {
		t.Fatalf("read source content: %v", err)
	}
	if string(raw) != readOnlyContent {
		t.Errorf("source content = %q, want %q", raw, readOnlyContent)
	}

	asset, err := source.OpenAsset("img-0001.png")
	if err != nil {
		t.Fatalf("OpenAsset: %v", err)
	}
	assetBytes, err := io.ReadAll(asset.File)
	asset.File.Close()
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if string(assetBytes) != "asset bytes" {
		t.Errorf("asset = %q, want %q", assetBytes, "asset bytes")
	}

	if _, err := s.GetAllLayers(); err != nil {
		t.Errorf("GetAllLayers: %v", err)
	}
	if _, err := s.ListTrashedBooks(); err != nil {
		t.Errorf("ListTrashedBooks: %v", err)
	}
	if _, err := s.Rescan(); err != nil {
		t.Errorf("Rescan: %v", err)
	}

	// Close is where a writable shelf flushes the exported book cache and the
	// directory scan snapshot, so the comparison has to happen after it.
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	closed = true

	after := treeSnapshot(t, libRoot)
	if diff := snapshotDiff(before, after); diff != "" {
		t.Errorf("a read-only shelf changed the shelf on disk:\n%s", diff)
	}
}

// snapshotDiff reports what the second walk saw that the first did not.
func snapshotDiff(before, after map[string]string) string {
	var lines []string
	for pth, state := range after {
		previous, existed := before[pth]
		switch {
		case !existed:
			lines = append(lines, fmt.Sprintf("+ %s (%s)", pth, state))
		case previous != state:
			lines = append(lines, fmt.Sprintf("~ %s (%s -> %s)", pth, previous, state))
		}
	}
	for pth := range before {
		if _, ok := after[pth]; !ok {
			lines = append(lines, fmt.Sprintf("- %s", pth))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// Every shelf-level mutation is refused with fsutil.ErrReadOnly, and refused
// before it moves anything: the ones that operate on an existing book would
// otherwise leave the book somewhere else on a shelf that cannot be repaired.
func TestReadOnlyShelfRefusesMutations(t *testing.T) {
	libRoot := t.TempDir()
	bookID := seedReadOnlyShelf(t, libRoot)

	before := treeSnapshot(t, libRoot)
	denyWrites(t, libRoot)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, ReadOnly: true})

	book, err := s.GetBook(bookID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}

	mutations := map[string]func() error{
		"NewBook":            func() error { _, err := s.NewBook(Layers{"fiction"}, "New"); return err },
		"MoveBook":           func() error { _, err := s.MoveBook(bookID, Layers{"moved"}); return err },
		"DeleteBook":         func() error { return s.DeleteBook(bookID) },
		"NewLayer":           func() error { return s.NewLayer(Layers{"added"}) },
		"DeleteLayer":        func() error { return s.DeleteLayer(Layers{"fiction"}) },
		"RenameLayer":        func() error { return s.RenameLayer(Layers{"fiction"}, Layers{"renamed"}) },
		"MoveLayer":          func() error { return s.MoveLayer(Layers{"fiction"}, Layers{}) },
		"RestoreTrashedBook": func() error { return s.RestoreTrashedBook(bookID) },
		"DeleteTrashedBook":  func() error { return s.DeleteTrashedBook(bookID) },
		"Book.SetMeta":       func() error { return book.SetMeta(book.GetMeta()) },
		"Book.NewSource":     func() error { _, err := book.NewSource(strings.NewReader("x")); return err },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := mutate(); !errors.Is(err, fsutil.ErrReadOnly) {
				t.Errorf("%s error = %v, want %v", name, err, fsutil.ErrReadOnly)
			}
		})
	}

	// The export is the one write that is not reached through writeRoot: it is
	// disabled by the writer ID read_only clears.
	if _, err := s.ExportBookCache(); err == nil {
		t.Error("ExportBookCache() succeeded on a read-only shelf")
	}

	if diff := snapshotDiff(before, treeSnapshot(t, libRoot)); diff != "" {
		t.Errorf("refused mutations changed the shelf on disk:\n%s", diff)
	}
}

// A shelf copied out of a backup, or exported by a reader that never wrote to
// it, has no app/ and no trash/. Opening it read-only must not need either.
func TestReadOnlyShelfWithoutRuntimeFoldersOpens(t *testing.T) {
	libRoot := t.TempDir()
	bookID := seedReadOnlyShelf(t, libRoot)

	for _, folder := range []string{appFolder, trashFolder} {
		if err := os.RemoveAll(filepath.Join(libRoot, folder)); err != nil {
			t.Fatalf("RemoveAll(%q): %v", folder, err)
		}
	}

	before := treeSnapshot(t, libRoot)
	denyWrites(t, libRoot)

	s := newTestShelf(t, &ShelfConf{LibRoot: libRoot, ReadOnly: true})

	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if len(books) != 1 || books[0].ID() != bookID {
		t.Fatalf("ListBooks returned %d books, want the seeded %q", len(books), bookID)
	}
	trashed, err := s.ListTrashedBooks()
	if err != nil {
		t.Fatalf("ListTrashedBooks: %v", err)
	}
	if len(trashed) != 0 {
		t.Errorf("ListTrashedBooks returned %d books, want none", len(trashed))
	}

	if diff := snapshotDiff(before, treeSnapshot(t, libRoot)); diff != "" {
		t.Errorf("opening a shelf without app/ or trash/ changed it:\n%s", diff)
	}
}

// lib_root is created for a writable shelf and never for a read-only one: the
// creation is itself a write, and a mount that is not there yet is far more
// likely to be a wrong path than a shelf to be conjured up.
func TestReadOnlyShelfDoesNotCreateLibRoot(t *testing.T) {
	libRoot := filepath.Join(t.TempDir(), "not-mounted")

	s, err := NewShelf(&ShelfConf{LibRoot: libRoot, ReadOnly: true})
	if err == nil {
		s.Close()
		t.Fatal("NewShelf on a missing lib_root succeeded, want an error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("NewShelf error = %v, want it to report the missing path", err)
	}
	if _, statErr := os.Stat(libRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("lib_root was created: Stat error = %v, want it to still not exist", statErr)
	}
}
