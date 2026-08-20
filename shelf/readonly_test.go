package shelf

import (
	"errors"
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
