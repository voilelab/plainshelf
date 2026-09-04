package bookpkg

import (
	"errors"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// newBookWithRawMeta writes bytes straight into a book package's book.json,
// bypassing the encoder. Every case below is a file no writer of this build
// would produce - it is what a text editor leaves behind - so the fixture has
// to be written as bytes rather than marshalled from a struct.
func newBookWithRawMeta(t *testing.T, raw string) (fsutil.FS, string) {
	t.Helper()

	tmpLib := t.TempDir()
	const bookFolder = "hand-edited.bookpkg"
	if err := os.MkdirAll(path.Join(tmpLib, bookFolder), 0o755); err != nil {
		t.Fatalf("Failed to create book dir: %v", err)
	}
	if err := os.WriteFile(path.Join(tmpLib, bookFolder, BookMetaFile), []byte(raw), 0o644); err != nil {
		t.Fatalf("Failed to write book.json: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	t.Cleanup(func() { tmpRoot.Close() })

	return fsutil.NewRootFS(tmpRoot), bookFolder
}

// A duplicate member is rejected rather than silently resolved to the last one,
// and the refusal has to be actionable: the decoder names the member, this
// package names the file.
func TestOpenBookRejectsDuplicateMemberAndNamesTheFileAndMember(t *testing.T) {
	root, bookFolder := newBookWithRawMeta(t, `{
  "schema_version": 1,
  "id": "hand-edited",
  "title": "First",
  "title": "Second"
}`)

	_, err := Open(root, newLoggerForTest(), bookFolder)
	if err == nil {
		t.Fatal("Expected opening a book with a duplicate member to fail, got no error")
	}

	if !errors.Is(err, ErrMalformedMetadata) {
		t.Errorf("errors.Is(err, ErrMalformedMetadata) = false, err = %v", err)
	}

	var malformed *MalformedMetadataError
	if !errors.As(err, &malformed) {
		t.Fatalf("errors.As(err, *MalformedMetadataError) = false, err = %v", err)
	}
	if want := path.Join(bookFolder, BookMetaFile); malformed.File != want {
		t.Errorf("File = %q, want %q", malformed.File, want)
	}
	if got := err.Error(); !strings.Contains(got, `"title"`) {
		t.Errorf("error does not name the duplicated member: %s", got)
	}
	if got := err.Error(); !strings.Contains(got, BookMetaFile) {
		t.Errorf("error does not name the file: %s", got)
	}
}

// Invalid UTF-8 is refused for the same reason, and the decoder points at the
// member holding it.
func TestOpenBookRejectsInvalidUTF8AndNamesTheFile(t *testing.T) {
	root, bookFolder := newBookWithRawMeta(t,
		"{\"schema_version\": 1, \"id\": \"hand-edited\", \"comments\": \"\xff\xfe\"}")

	_, err := Open(root, newLoggerForTest(), bookFolder)
	if err == nil {
		t.Fatal("Expected opening a book with invalid UTF-8 to fail, got no error")
	}
	if !errors.Is(err, ErrMalformedMetadata) {
		t.Errorf("errors.Is(err, ErrMalformedMetadata) = false, err = %v", err)
	}
	if got := err.Error(); !strings.Contains(got, BookMetaFile) {
		t.Errorf("error does not name the file: %s", got)
	}
}

// Trailing data after the first value is refused too: a half-finished edit that
// leaves a second object behind is not a book this build guesses at.
func TestOpenBookRejectsTrailingData(t *testing.T) {
	root, bookFolder := newBookWithRawMeta(t,
		`{"schema_version": 1, "id": "hand-edited", "title": "First"} {"title": "Second"}`)

	if _, err := Open(root, newLoggerForTest(), bookFolder); !errors.Is(err, ErrMalformedMetadata) {
		t.Errorf("errors.Is(err, ErrMalformedMetadata) = false, err = %v", err)
	}
}

// Member names are matched case-sensitively, so "Title" is not the title
// field - it is an unknown member, which is read without complaint and, until
// the passthrough PSW-93 adds, dropped by the next write to the book.
//
// This case pins the read half only. That a hand-added key survives a write is
// PSW-93's to deliver and is not asserted here.
func TestOpenBookDoesNotApplyACaseVariantMemberName(t *testing.T) {
	root, bookFolder := newBookWithRawMeta(t,
		`{"schema_version": 1, "id": "hand-edited", "Title": "Capitalised"}`)

	book, err := Open(root, newLoggerForTest(), bookFolder)
	if err != nil {
		t.Fatalf("Expected an unknown member to be accepted, got %v", err)
	}
	if got := book.Title(); got != "" {
		t.Errorf("Title() = %q, want %q: a case variant must not be read as the title", got, "")
	}
}

// A source's meta.json is read with the same strictness, and names itself
// rather than the book that holds it.
func TestOpenSourceRejectsDuplicateMemberAndNamesTheFile(t *testing.T) {
	tmpLib := t.TempDir()
	const sourceFolder = "20260315-a1"
	if err := os.MkdirAll(path.Join(tmpLib, sourceFolder), 0o755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	raw := `{"schema_version": 1, "format": "txt", "format": "md"}`
	if err := os.WriteFile(path.Join(tmpLib, sourceFolder, SourceMetaFile), []byte(raw), 0o644); err != nil {
		t.Fatalf("Failed to write meta.json: %v", err)
	}
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	t.Cleanup(func() { tmpRoot.Close() })

	_, err = openSource(fsutil.NewRootFS(tmpRoot), sourceFolder)
	if err == nil {
		t.Fatal("Expected opening a source with a duplicate member to fail, got no error")
	}

	var malformed *MalformedMetadataError
	if !errors.As(err, &malformed) {
		t.Fatalf("errors.As(err, *MalformedMetadataError) = false, err = %v", err)
	}
	if want := path.Join(sourceFolder, SourceMetaFile); malformed.File != want {
		t.Errorf("File = %q, want %q", malformed.File, want)
	}
	if got := err.Error(); !strings.Contains(got, `"format"`) {
		t.Errorf("error does not name the duplicated member: %s", got)
	}
}
