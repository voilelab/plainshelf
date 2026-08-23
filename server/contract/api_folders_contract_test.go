package contract_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

func TestAPIFolderMoveAndRenameContract(t *testing.T) {
	env := newAPITestEnv(t)
	created := importTextBook(t, env, "Folder Ops", "alpha/beta", "folder.txt", "body")

	rec := env.post(shelfURL("folders", "gamma"), nil)
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.post(shelfURL("folder-moves"), strings.NewReader(`{"folder":["alpha","beta"],"target_folder":["gamma"]}`))
	assertStatus(t, rec, http.StatusNoContent)

	rec = env.patch(shelfURL("folders", "gamma", "beta"), strings.NewReader(`{"name":"renamed"}`))
	assertStatus(t, rec, http.StatusNoContent)

	got := getJSON[server.Book](t, env, bookURL(created.Meta.ID))
	if strings.Join(got.Folder, "/") != "gamma/renamed" {
		t.Fatalf("folder = %#v, want gamma/renamed", got.Folder)
	}

	// Moving onto a folder that does not exist is a conflict, and a name that is
	// not a single path segment is a client error.
	rec = env.post(shelfURL("folder-moves"), strings.NewReader(`{"folder":["gamma","renamed"],"target_folder":["missing"]}`))
	assertStatus(t, rec, http.StatusConflict)

	rec = env.patch(shelfURL("folders", "gamma", "renamed"), strings.NewReader(`{"name":"bad/name"}`))
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestAPIInvalidFolderNameIsARequestErrorContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Folder Book", "keep", "folder.txt", "body")

	tests := []struct {
		name         string
		method, path string
		body         string
	}{
		{
			name:   "create folder",
			method: http.MethodPost,
			path:   shelfURL("folders", "bad%2F..%2Fesc"),
		},
		{
			name:   "move folder",
			method: http.MethodPost,
			path:   shelfURL("folder-moves"),
			body:   `{"folder":[".."],"target_folder":["beta"]}`,
		},
		{
			name:   "move book to folder",
			method: http.MethodPatch,
			path:   bookURL(book.Meta.ID),
			body:   `{"folder":[".."]}`,
		},
		{
			// Creating and importing a book place it in a folder too, so they
			// reject a bad one on the same terms.
			name:   "create book in folder",
			method: http.MethodPost,
			path:   booksURL(),
			body:   `{"title":"X","folder":[".."]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.request(tt.method, tt.path, strings.NewReader(tt.body))

			assertStatus(t, rec, http.StatusBadRequest)
			if got := strings.TrimSpace(rec.Body.String()); got != "invalid folder name" {
				t.Fatalf("body = %q, want %q", got, "invalid folder name")
			}
		})
	}
}

// The folder routes answer outcomes the error table cannot name as a conflict.
// Routing them through the table must not turn those into 500s.
func TestAPIFolderConflictsStillAnswerConflictContract(t *testing.T) {
	env := newAPITestEnv(t)

	for _, folder := range []string{"alpha", "beta"} {
		assertStatus(t, env.post(shelfURL("folders", folder), nil), http.StatusNoContent)
	}

	t.Run("rename onto an existing folder", func(t *testing.T) {
		rec := env.patch(shelfURL("folders", "alpha"), strings.NewReader(`{"name":"beta"}`))
		assertStatus(t, rec, http.StatusConflict)
	})

	t.Run("move a folder under itself", func(t *testing.T) {
		rec := env.post(shelfURL("folder-moves"),
			strings.NewReader(`{"folder":["alpha"],"target_folder":["alpha"]}`))
		assertStatus(t, rec, http.StatusConflict)
	})
}

// Importing places the book in a folder as well, so a bad one must be refused
// the same way rather than reported as an import failure.
func TestAPIImportRejectsInvalidFolderNameContract(t *testing.T) {
	env := newAPITestEnv(t)

	rec := postBookImport(t, env,
		bookUpload("import.txt", plainTextContentType, "body", [2]string{"folder", ".."}))

	assertStatus(t, rec, http.StatusBadRequest)
	if got := strings.TrimSpace(rec.Body.String()); got != "invalid folder name" {
		t.Fatalf("body = %q, want %q", got, "invalid folder name")
	}
}

// A folder named after a directory the scanners skip is refused for a reason the
// user cannot guess from "invalid folder name": the folder would be created and
// then never listed again. The shelf error already explains it, so the API must
// carry that reason out instead of flattening every rejection into one message.
func TestAPIIgnoredFolderNameExplainsTheRuleContract(t *testing.T) {
	env := newAPITestEnv(t)
	book := importTextBook(t, env, "Ignored Folder Book", "keep", "folder.txt", "body")

	ignored := []struct {
		name         string
		method, path string
		body         string
	}{
		{
			name:   "create folder",
			method: http.MethodPost,
			path:   shelfURL("folders", "%40eaDir"),
		},
		{
			name:   "create book in folder",
			method: http.MethodPost,
			path:   booksURL(),
			body:   `{"title":"X","folder":[".backup"]}`,
		},
		{
			name:   "move book to folder",
			method: http.MethodPatch,
			path:   bookURL(book.Meta.ID),
			body:   `{"folder":["lost+found"]}`,
		},
	}

	for _, tt := range ignored {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.request(tt.method, tt.path, strings.NewReader(tt.body))

			assertStatus(t, rec, http.StatusBadRequest)
			got := strings.TrimSpace(rec.Body.String())
			if got == "invalid folder name" {
				t.Fatalf("body = %q, want a message naming the ignore rule", got)
			}
			for _, want := range []string{"@eaDir", "skipped by the shelf scanner"} {
				if !strings.Contains(got, want) {
					t.Fatalf("body = %q, want it to contain %q", got, want)
				}
			}
		})
	}

	// A folder that is invalid for any other reason keeps the general message,
	// so the specific entry must not swallow the table's existing one.
	t.Run("other invalid folder keeps the general message", func(t *testing.T) {
		rec := env.post(shelfURL("folders", "bad%2F..%2Fesc"), nil)

		assertStatus(t, rec, http.StatusBadRequest)
		if got := strings.TrimSpace(rec.Body.String()); got != "invalid folder name" {
			t.Fatalf("body = %q, want %q", got, "invalid folder name")
		}
	})
}
