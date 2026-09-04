package contract_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

func TestAPIFolderMoveAndRenameContract(t *testing.T) {
	env := New(t)
	created := ImportTextBook(t, env, "Folder Ops", "alpha/beta", "folder.txt", "body")

	rec := env.Post(ShelfURL("folders", "gamma"), nil)
	AssertStatus(t, rec, http.StatusNoContent)

	rec = env.Post(ShelfURL("folder-moves"), strings.NewReader(`{"folder":["alpha","beta"],"target_folder":["gamma"]}`))
	AssertStatus(t, rec, http.StatusNoContent)

	rec = env.Patch(ShelfURL("folders", "gamma", "beta"), strings.NewReader(`{"name":"renamed"}`))
	AssertStatus(t, rec, http.StatusNoContent)

	got := GetJSON[server.Book](t, env, BookURL(created.Meta.ID))
	if strings.Join(got.Folder, "/") != "gamma/renamed" {
		t.Fatalf("folder = %#v, want gamma/renamed", got.Folder)
	}

	// Moving onto a folder that does not exist is a conflict, and a name that is
	// not a single path segment is a client error.
	rec = env.Post(ShelfURL("folder-moves"), strings.NewReader(`{"folder":["gamma","renamed"],"target_folder":["missing"]}`))
	AssertStatus(t, rec, http.StatusConflict)

	rec = env.Patch(ShelfURL("folders", "gamma", "renamed"), strings.NewReader(`{"name":"bad/name"}`))
	AssertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIInvalidFolderNameIsARequestErrorContract(t *testing.T) {
	env := New(t)
	book := ImportTextBook(t, env, "Folder Book", "keep", "folder.txt", "body")

	tests := []struct {
		name         string
		method, path string
		body         string
	}{
		{
			name:   "create folder",
			method: http.MethodPost,
			path:   ShelfURL("folders", "bad%2F..%2Fesc"),
		},
		{
			name:   "move folder",
			method: http.MethodPost,
			path:   ShelfURL("folder-moves"),
			body:   `{"folder":[".."],"target_folder":["beta"]}`,
		},
		{
			name:   "move book to folder",
			method: http.MethodPatch,
			path:   BookURL(book.Meta.ID),
			body:   `{"folder":[".."]}`,
		},
		{
			// Creating and importing a book place it in a folder too, so they
			// reject a bad one on the same terms.
			name:   "create book in folder",
			method: http.MethodPost,
			path:   BooksURL(),
			body:   `{"title":"X","folder":[".."]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.Request(tt.method, tt.path, strings.NewReader(tt.body))

			AssertErrorEnvelope(t, rec, http.StatusBadRequest,
				"INVALID_FOLDER", "invalid folder name")
		})
	}
}

// The folder routes answer outcomes the error table cannot name as a conflict.
// Routing them through the table must not turn those into 500s.
func TestAPIFolderConflictsStillAnswerConflictContract(t *testing.T) {
	env := New(t)

	for _, folder := range []string{"alpha", "beta"} {
		AssertStatus(t, env.Post(ShelfURL("folders", folder), nil), http.StatusNoContent)
	}

	t.Run("rename onto an existing folder", func(t *testing.T) {
		rec := env.Patch(ShelfURL("folders", "alpha"), strings.NewReader(`{"name":"beta"}`))
		AssertStatus(t, rec, http.StatusConflict)
	})

	t.Run("move a folder under itself", func(t *testing.T) {
		rec := env.Post(ShelfURL("folder-moves"),
			strings.NewReader(`{"folder":["alpha"],"target_folder":["alpha"]}`))
		AssertStatus(t, rec, http.StatusConflict)
	})
}

// Importing places the book in a folder as well, so a bad one must be refused
// the same way rather than reported as an import failure.
func TestAPIImportRejectsInvalidFolderNameContract(t *testing.T) {
	env := New(t)

	rec := PostBookImport(t, env,
		BookUpload("import.txt", PlainTextContentType, "body", [2]string{"folder", ".."}))

	AssertErrorEnvelope(t, rec, http.StatusBadRequest,
		"INVALID_FOLDER", "invalid folder name")
}

// A folder named after a directory the scanners skip is refused for a reason the
// user cannot guess from "invalid folder name": the folder would be created and
// then never listed again. The shelf error already explains it, so the API must
// carry that reason out instead of flattening every rejection into one message.
func TestAPIIgnoredFolderNameExplainsTheRuleContract(t *testing.T) {
	env := New(t)
	book := ImportTextBook(t, env, "Ignored Folder Book", "keep", "folder.txt", "body")

	ignored := []struct {
		name         string
		method, path string
		body         string
	}{
		{
			name:   "create folder",
			method: http.MethodPost,
			path:   ShelfURL("folders", "%40eaDir"),
		},
		{
			name:   "create book in folder",
			method: http.MethodPost,
			path:   BooksURL(),
			body:   `{"title":"X","folder":[".backup"]}`,
		},
		{
			name:   "move book to folder",
			method: http.MethodPatch,
			path:   BookURL(book.Meta.ID),
			body:   `{"folder":["lost+found"]}`,
		},
	}

	for _, tt := range ignored {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.Request(tt.method, tt.path, strings.NewReader(tt.body))

			AssertStatus(t, rec, http.StatusBadRequest)
			got := DecodeJSON[ErrorEnvelope](t, rec).Error
			if got.Code != "IGNORED_FOLDER_NAME" {
				t.Fatalf("error code = %q, want %q", got.Code, "IGNORED_FOLDER_NAME")
			}
			if got.Message == "invalid folder name" {
				t.Fatalf("message = %q, want one naming the ignore rule", got.Message)
			}
			for _, want := range []string{"while scanning", "would not stay visible"} {
				if !strings.Contains(got.Message, want) {
					t.Fatalf("message = %q, want it to contain %q", got.Message, want)
				}
			}
		})
	}

	// A folder that is invalid for any other reason keeps the general message,
	// so the specific entry must not swallow the table's existing one.
	t.Run("other invalid folder keeps the general message", func(t *testing.T) {
		rec := env.Post(ShelfURL("folders", "bad%2F..%2Fesc"), nil)

		AssertErrorEnvelope(t, rec, http.StatusBadRequest,
			"INVALID_FOLDER", "invalid folder name")
	})
}

// A shelf that lists its own directories also says why it skips them, and that
// reason is what the user needs: they wrote the rule and can take it back. The
// message therefore carries the shelf's own words rather than the built-in
// directory names, which on such a shelf are not the rule in force.
func TestAPIConfiguredIgnoredFolderNameCarriesTheShelfsReasonContract(t *testing.T) {
	libRoot := t.TempDir()
	config := `{"schema_version":1,"scan":{"ignored_dirs":[{"name":"@Snapshot","reason":"Synology snapshot directory"}]}}`
	if err := os.WriteFile(filepath.Join(libRoot, "shelf.json"), []byte(config), 0644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}

	env := New(t, WithLibRoot(libRoot))

	rec := env.Post(ShelfURL("folders", "%40Snapshot"), nil)

	AssertStatus(t, rec, http.StatusBadRequest)
	got := DecodeJSON[ErrorEnvelope](t, rec).Error.Message
	for _, want := range []string{"@Snapshot", "Synology snapshot directory"} {
		if !strings.Contains(got, want) {
			t.Fatalf("message = %q, want it to contain %q", got, want)
		}
	}

	// The directory must not have been created on the way to the rejection.
	if _, err := os.Stat(filepath.Join(libRoot, "books", "@Snapshot")); !os.IsNotExist(err) {
		t.Errorf("Stat books/@Snapshot = %v, want it never to have been created", err)
	}
}
