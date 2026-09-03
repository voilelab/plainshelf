package contract_test

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every refusal writeErr answers carries the same JSON envelope, so a client
// can read one shape rather than parsing prose. The code is the stable half:
// the message may be reworded, the code may not.
func TestAPIErrorsAnswerAJSONEnvelopeContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Envelope Book", "", "envelope.txt", "body")

	tests := []struct {
		name         string
		method, path string
		body         string
		status       int
		code         string
		message      string
	}{
		{
			name:    "unknown book",
			method:  http.MethodDelete,
			path:    bookURL("no_such_book"),
			status:  http.StatusNotFound,
			code:    "BOOK_NOT_FOUND",
			message: "book not found",
		},
		{
			name:    "unknown source",
			method:  http.MethodDelete,
			path:    sourceURL(book.Meta.ID, "no_such_source"),
			status:  http.StatusNotFound,
			code:    "SOURCE_NOT_FOUND",
			message: "source not found",
		},
		{
			name:    "unknown trashed book",
			method:  http.MethodPost,
			path:    shelfURL("trash", "books", "no_such_book", "restore"),
			status:  http.StatusNotFound,
			code:    "TRASHED_BOOK_NOT_FOUND",
			message: "trashed book not found",
		},
		{
			name:    "star out of range",
			method:  http.MethodPatch,
			path:    bookURL(book.Meta.ID),
			body:    `{"star":9}`,
			status:  http.StatusBadRequest,
			code:    "INVALID_STAR",
			message: "star must be between 0 and 5",
		},
		{
			name:    "invalid folder name",
			method:  http.MethodPost,
			path:    shelfURL("folders", "bad%2F..%2Fesc"),
			status:  http.StatusBadRequest,
			code:    "INVALID_FOLDER",
			message: "invalid folder name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.request(tt.method, tt.path, strings.NewReader(tt.body))

			assertErrorEnvelope(t, rec, tt.status, tt.code, tt.message)

			// A client reading the body line-wise would block without it, the
			// same reason the success bodies carry one.
			if got := rec.Body.String(); !strings.HasSuffix(got, "\n") {
				t.Fatalf("body = %q, want a trailing newline", got)
			}
		})
	}
}

// A refusal the shelf itself raises keeps its Retry-After alongside the
// envelope: the code says what happened, the header says when to come back.
func TestAPIReadOnlyShelfRefusalCarriesItsCodeContract(t *testing.T) {
	env := newAPITestEnv(t, withReadOnlyShelf())

	rec := env.post(shelfURL("folders", "new-folder"), nil)

	assertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
		"shelf is opened read-only; this PlainShelf instance cannot modify it")
}

// A 500 says only what the caller was trying to do. The cause names the shelf's
// on-disk layout - the book's directory, the atomic write's temp file - and
// that stays in the log, because a user pastes an error body into a bug report
// without reading it first.
func TestAPIInternalErrorBodyWithholdsTheCauseContract(t *testing.T) {
	// The shelf root is named distinctively so a leaked path is unmistakable
	// rather than a substring that could plausibly come from anywhere.
	libRoot := filepath.Join(t.TempDir(), "secret-shelf-b7f2")
	if err := os.MkdirAll(libRoot, 0755); err != nil {
		t.Fatalf("create shelf root: %v", err)
	}

	env := newAPITestEnv(t, withLibRoot(libRoot))
	book := importTextBook(t, env, "secret-manuscript", "", "secret-manuscript.txt", "body")

	sources := getJSON[[]struct {
		ID string `json:"id"`
	}](t, env, bookURL(book.Meta.ID, "sources"))
	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}

	// Replacing the source file with a directory makes the atomic rename fail,
	// which is an error the table does not name: exactly the fallback path. The
	// file is located by walking rather than by rebuilding the shelf's naming
	// rule, which is not what this test is about.
	sourceFile := findSourceFile(t, libRoot)
	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("remove source file: %v", err)
	}
	if err := os.Mkdir(sourceFile, 0755); err != nil {
		t.Fatalf("replace source file with a directory: %v", err)
	}

	rec := env.patchContent(sourceURL(book.Meta.ID, sources[0].ID, "content"),
		plainTextContentType, strings.NewReader("replacement"))

	assertErrorEnvelope(t, rec, http.StatusInternalServerError, "INTERNAL",
		"failed to update book source content")

	body := rec.Body.String()
	for _, secret := range []string{"secret-shelf-b7f2", "secret-manuscript", ".tmp", "renameat"} {
		if strings.Contains(body, secret) {
			t.Fatalf("body = %q, must not carry %q out of the log", body, secret)
		}
	}
}

// findSourceFile returns the single source file under a shelf holding one book.
func findSourceFile(t *testing.T, libRoot string) string {
	t.Helper()

	var found []string
	err := filepath.WalkDir(libRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "source.") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk shelf root: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("source files = %v, want exactly one", found)
	}
	return found[0]
}
