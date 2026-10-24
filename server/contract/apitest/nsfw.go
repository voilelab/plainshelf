package apitest

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
)

const (
	NSFWMarkedFolder  = "Fiction/Adult"
	NSFWFlaggedFolder = "Fiction/Flagged"
	NSFWEmptyFolder   = "Fiction/Empty"

	NSFWMarkedReason = "kept apart from the rest of the shelf"

	nsfwShelfJSON = `{"schema_version":1,"content":{"nsfw_folders":[{"path":"` + NSFWMarkedFolder +
		`","reason":"` + NSFWMarkedReason + `"}]}}`
)

const nsfwTwinProse = "the quick brown fox jumps over the lazy dog while the industrious " +
	"bee gathers nectar from a thousand summer flowers and the river winds " +
	"slowly toward the distant glimmering sea beneath an indifferent sky " +
	"the quick brown fox jumps over the lazy dog while the industrious " +
	"bee gathers nectar from a thousand summer flowers and the river winds " +
	"slowly toward the distant glimmering sea beneath an indifferent sky "

type NSFWShelf struct {
	Env *Env

	Visible, Classic, FolderHidden, BookHidden string
}

func (s NSFWShelf) Hidden() []string { return []string{s.FolderHidden, s.BookHidden} }
func (s NSFWShelf) All() []string {
	return []string{s.Visible, s.Classic, s.FolderHidden, s.BookHidden}
}

func NewNSFWShelf(t *testing.T, opts ...Option) NSFWShelf {
	t.Helper()

	libRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(libRoot, "shelf.json"), []byte(nsfwShelfJSON), 0o644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}
	env := New(t, append([]Option{WithLibRoot(libRoot)}, opts...)...)

	book := func(title, folder, body string) string {
		return ImportTextBook(t, env, title, folder, title+".txt", body).Meta.ID
	}
	shelf := NSFWShelf{
		Env:          env,
		Visible:      book("Visible", "", nsfwTwinProse),
		Classic:      book("Classic", "Fiction/Classics", "unrelated vocabulary weaving mercury zephyr quartz lantern"),
		FolderHidden: book("FolderHidden", NSFWMarkedFolder, nsfwTwinProse),
		BookHidden:   book("BookHidden", NSFWFlaggedFolder, "another vocabulary entirely obsidian marigold trestle vellum"),
	}

	AssertStatus(t, env.Post(ShelfURL("folders", NSFWEmptyFolder), nil), http.StatusNoContent)
	EditBookMetaFile(t, env, shelf.BookHidden, map[string]any{"nsfw": true})
	AssertStatus(t, env.Post(ScansURL(), nil), http.StatusOK)
	return shelf
}

func SetBookNSFW(t *testing.T, env *Env, bookID string, nsfw bool) server.Book {
	t.Helper()

	rec := env.Patch(BookURL(bookID), strings.NewReader(`{"nsfw":`+strconv.FormatBool(nsfw)+`}`))
	AssertStatus(t, rec, http.StatusOK)
	return DecodeJSON[server.Book](t, rec)
}

func SetShowNSFW(t *testing.T, env *Env, show bool) {
	t.Helper()

	rec := env.Post(SettingURL("show_nsfw"), strings.NewReader(strconv.FormatBool(show)))
	AssertStatus(t, rec, http.StatusNoContent)
}
