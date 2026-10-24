package crosscut_test

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server/contract/apitest"
)

func TestAPIErrorsAnswerAJSONEnvelopeContract(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Envelope Book", "", "envelope.txt", "body")

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
			path:    apitest.BookURL("no_such_book"),
			status:  http.StatusNotFound,
			code:    "BOOK_NOT_FOUND",
			message: "book not found",
		},
		{
			name:    "unknown source",
			method:  http.MethodDelete,
			path:    apitest.SourceURL(book.Meta.ID, "no_such_source"),
			status:  http.StatusNotFound,
			code:    "SOURCE_NOT_FOUND",
			message: "source not found",
		},
		{
			name:    "unknown trashed book",
			method:  http.MethodPost,
			path:    apitest.ShelfURL("trash", "books", "no_such_book", "restore"),
			status:  http.StatusNotFound,
			code:    "TRASHED_BOOK_NOT_FOUND",
			message: "trashed book not found",
		},
		{
			name:    "star out of range",
			method:  http.MethodPatch,
			path:    apitest.BookURL(book.Meta.ID),
			body:    `{"star":9}`,
			status:  http.StatusBadRequest,
			code:    "INVALID_STAR",
			message: "star must be between 0 and 5",
		},
		{
			name:    "invalid folder name",
			method:  http.MethodPost,
			path:    apitest.ShelfURL("folders", "bad%2F..%2Fesc"),
			status:  http.StatusBadRequest,
			code:    "INVALID_FOLDER",
			message: "invalid folder name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.Request(tt.method, tt.path, strings.NewReader(tt.body))

			apitest.AssertErrorEnvelope(t, rec, tt.status, tt.code, tt.message)

			if got := rec.Body.String(); !strings.HasSuffix(got, "\n") {
				t.Fatalf("body = %q, want a trailing newline", got)
			}
		})
	}
}

func TestAPIReadOnlyShelfRefusalCarriesItsCodeContract(t *testing.T) {
	env := apitest.New(t, apitest.WithReadOnlyShelf())

	rec := env.Post(apitest.ShelfURL("folders", "new-folder"), nil)

	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "SHELF_READ_ONLY",
		"shelf is opened read-only; this PlainShelf instance cannot modify it")
}

func TestAPIInternalErrorBodyWithholdsTheCauseContract(t *testing.T) {

	libRoot := filepath.Join(t.TempDir(), "secret-shelf-b7f2")
	if err := os.MkdirAll(libRoot, 0755); err != nil {
		t.Fatalf("create shelf root: %v", err)
	}

	env := apitest.New(t, apitest.WithLibRoot(libRoot))
	book := apitest.ImportTextBook(t, env, "secret-manuscript", "", "secret-manuscript.txt", "body")

	sources := apitest.GetJSON[[]struct {
		ID string `json:"id"`
	}](t, env, apitest.BookURL(book.Meta.ID, "sources"))
	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}

	sourceFile := findSourceFile(t, libRoot)
	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("remove source file: %v", err)
	}
	if err := os.Mkdir(sourceFile, 0755); err != nil {
		t.Fatalf("replace source file with a directory: %v", err)
	}

	rec := env.PatchContent(apitest.SourceURL(book.Meta.ID, sources[0].ID, "content"),
		apitest.PlainTextContentType, strings.NewReader("replacement"))

	apitest.AssertErrorEnvelope(t, rec, http.StatusInternalServerError, "INTERNAL",
		"failed to update book source content")

	body := rec.Body.String()
	for _, secret := range []string{"secret-shelf-b7f2", "secret-manuscript", ".tmp", "renameat"} {
		if strings.Contains(body, secret) {
			t.Fatalf("body = %q, must not carry %q out of the log", body, secret)
		}
	}
}

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
