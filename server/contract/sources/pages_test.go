package sources_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

const invalidPageListMessage = "every page must name an image file directly inside assets/"

func createImagesSource(t *testing.T, env *apitest.Env, bookID, pageList string) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{"content": pageList, "format": "images", "set_current": true})
	if err != nil {
		t.Fatal(err)
	}
	rec := env.RequestTyped(http.MethodPost, apitest.BookURL(bookID, "sources"),
		"application/json", bytes.NewReader(body))
	apitest.AssertStatus(t, rec, http.StatusOK)

	created := apitest.DecodeJSON[map[string]any](t, rec)
	if created["format"] != "images" {
		t.Fatalf("created source = %#v, want format images", created)
	}
	if _, ok := created["page_direction"]; ok {
		t.Fatalf("created source = %#v, want no page_direction for the default", created)
	}
	if created["char_count"] != nil {
		t.Fatalf("created source = %#v, want no char_count for an images source", created)
	}
	sourceID, _ := created["id"].(string)
	return sourceID
}

func TestAPIImagesSourcePagesContract(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Night Train", "", "night.txt", "prose first")
	sourceID := createImagesSource(t, env, book.Meta.ID, "p-b.png\np-a.jpg\n")

	activated := apitest.GetJSON[server.Book](t, env, apitest.BookURL(book.Meta.ID))
	if activated.Meta == nil || activated.Meta.Format != shelf.BookFormatImages {
		t.Fatalf("book = %#v, want format images mirrored from the current source", activated.Meta)
	}

	env.WriteSourceAsset(t, book.Meta.ID, sourceID, "p-a.jpg", []byte("a"))
	env.WriteSourceAsset(t, book.Meta.ID, sourceID, "p-stray.gif", []byte("s"))

	got := apitest.GetJSON[shelf.PageList](t, env, apitest.SourceURL(book.Meta.ID, sourceID, "pages"))
	want := shelf.PageList{
		Pages:    []shelf.Page{{Name: "p-b.png", Missing: true}, {Name: "p-a.jpg"}},
		Unlisted: []string{"p-stray.gif"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pages = %+v, want %+v", got, want)
	}

	textSourceID := book.Meta.CurrentSource
	rec := env.Get(apitest.SourceURL(book.Meta.ID, textSourceID, "pages"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "NOT_IMAGE_SOURCE", "source is not an images source")

	rec = env.Get(apitest.SourceURL(book.Meta.ID, "no-such-source", "pages"))
	apitest.AssertStatus(t, rec, http.StatusNotFound)
}

func TestAPIImagesSourceRejectsBadPageList(t *testing.T) {
	env := apitest.New(t)
	book := apitest.ImportTextBook(t, env, "Night Train", "", "night.txt", "prose first")

	rec := env.RequestTyped(http.MethodPost, apitest.BookURL(book.Meta.ID, "sources"),
		"application/json", strings.NewReader(`{"content":"../escape.png\n","format":"images"}`))
	apitest.AssertErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_PAGE_LIST", invalidPageListMessage)

	sourceID := createImagesSource(t, env, book.Meta.ID, "p-a.png\n")
	rec = env.PatchContent(apitest.SourceURL(book.Meta.ID, sourceID, "content"),
		"text/plain; charset=utf-8", strings.NewReader("notes.txt\n"))
	apitest.AssertErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_PAGE_LIST", invalidPageListMessage)

	content := env.Get(apitest.SourceURL(book.Meta.ID, sourceID, "content"))
	apitest.AssertStatus(t, content, http.StatusOK)
	if got := content.Body.String(); got != "p-a.png\n" {
		t.Fatalf("page list = %q after a rejected update, want it untouched", got)
	}
}

func TestAPIFingerprintSkipsImagesBooks(t *testing.T) {
	env := apitest.New(t)
	prose := apitest.ImportTextBook(t, env, "Dune", "", "dune.txt", "the spice must flow")
	comic := apitest.ImportTextBook(t, env, "Night Train", "", "night.txt", "prose first")
	createImagesSource(t, env, comic.Meta.ID, "p-a.png\n")
	backdateSourcesForFingerprintCache(t, env, prose.Meta.ID)
	backdateSourcesForFingerprintCache(t, env, comic.Meta.ID)

	result := runFingerprintSources(t, env)
	if result.Sources != 2 || result.Fingerprinted != 2 || len(result.Failures) != 0 {
		t.Fatalf("fingerprint run = %+v, want the two text sources only", result)
	}

	status := apitest.GetJSON[fingerprintStatusResponse](t, env, apitest.FingerprintStatusURL())
	if status.Total != 1 || status.Fingerprinted != 1 || status.Missing != 0 {
		t.Fatalf("status = %+v, want the images book left out of coverage", status)
	}

	apitest.PatchBook(t, env, comic.Meta.ID, `{"format":"txt"}`, http.StatusOK)
	apitest.PatchBook(t, env, prose.Meta.ID, `{"format":"images"}`, http.StatusOK)
	status = apitest.GetJSON[fingerprintStatusResponse](t, env, apitest.FingerprintStatusURL())
	if status.Total != 1 || status.Fingerprinted != 1 {
		t.Fatalf("status after patching book.json format = %+v, want coverage to follow the current source", status)
	}
}
