package sources_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json/v2"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/testutil"
	"github.com/voilelab/plainshelf/server/contract/apitest"
	"github.com/voilelab/plainshelf/shelf"
)

const pageConflictMessage = "the page list changed since it was read; reload it and try again"

func newComicForEdits(t *testing.T, env *apitest.Env, pageList string, assets ...string) (string, string) {
	t.Helper()

	book := apitest.ImportTextBook(t, env, "Night Train", "", "night.txt", "prose first")
	sourceID := createImagesSource(t, env, book.Meta.ID, pageList)
	for _, name := range assets {
		env.WriteSourceAsset(t, book.Meta.ID, sourceID, name, []byte(name))
	}
	return book.Meta.ID, sourceID
}

func pagesOf(t *testing.T, env *apitest.Env, bookID, sourceID string) shelf.PageList {
	t.Helper()
	return apitest.GetJSON[shelf.PageList](t, env, apitest.SourceURL(bookID, sourceID, "pages"))
}

func putPages(env *apitest.Env, bookID, sourceID, md5Hash string, names ...string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"pages": names, "md5_hash": md5Hash})
	return env.RequestTyped(http.MethodPut, apitest.SourceURL(bookID, sourceID, "pages"),
		"application/json", bytes.NewReader(body))
}

func insertPageURL(bookID, sourceID, md5Hash string, position int) string {
	query := url.Values{"md5_hash": {md5Hash}, "position": {strconv.Itoa(position)}}
	return apitest.SourceURL(bookID, sourceID, "pages") + "?" + query.Encode()
}

func deletePageURL(bookID, sourceID, name, md5Hash string) string {
	return apitest.SourceURL(bookID, sourceID, "pages", name) + "?md5_hash=" + url.QueryEscape(md5Hash)
}

func names(list shelf.PageList) []string {
	out := make([]string, len(list.Pages))
	for i, page := range list.Pages {
		out[i] = page.Name
	}
	return out
}

func pngHeader(width, height uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8], ihdr[9] = 8, 2

	chunk := append([]byte("IHDR"), ihdr...)
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&out, binary.BigEndian, uint32(len(ihdr)))
	out.Write(chunk)
	_ = binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return out.Bytes()
}

func TestAPIReplacePagesContract(t *testing.T) {
	env := apitest.New(t)
	bookID, sourceID := newComicForEdits(t, env, "a.png\nb.png\n", "a.png", "b.png", "stray.gif")
	before := pagesOf(t, env, bookID, sourceID)

	rec := putPages(env, bookID, sourceID, before.MD5Hash, "b.png", "stray.gif", "a.png")
	apitest.AssertStatus(t, rec, http.StatusOK)
	got := apitest.DecodeJSON[shelf.PageList](t, rec)
	if want := []string{"b.png", "stray.gif", "a.png"}; !reflect.DeepEqual(names(got), want) || len(got.Unlisted) != 0 {
		t.Fatalf("pages = %+v, want %v with nothing unlisted", got, want)
	}
	if got.MD5Hash == before.MD5Hash {
		t.Error("md5_hash did not change with the page list")
	}

	content := env.Get(apitest.SourceURL(bookID, sourceID, "content"))
	if body := content.Body.String(); body != "b.png\nstray.gif\na.png\n" {
		t.Fatalf("source.txt = %q, want the new order", body)
	}

	rec = putPages(env, bookID, sourceID, before.MD5Hash, "a.png")
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "PAGE_LIST_CONFLICT", pageConflictMessage)

	rec = putPages(env, bookID, sourceID, "", "a.png")
	apitest.AssertStatus(t, rec, http.StatusBadRequest)

	rec = putPages(env, bookID, sourceID, got.MD5Hash, "../a.png")
	apitest.AssertErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_PAGE_LIST", invalidPageListMessage)
}

func TestAPIInsertPageContract(t *testing.T) {
	env := apitest.New(t)
	bookID, sourceID := newComicForEdits(t, env, "a.png\nb.png\n", "a.png", "b.png")
	before := pagesOf(t, env, bookID, sourceID)
	image := testutil.OnePixelPNG()

	rec := env.RequestTyped(http.MethodPost, insertPageURL(bookID, sourceID, before.MD5Hash, 1), "image/png", bytes.NewReader(image))
	apitest.AssertStatus(t, rec, http.StatusOK)
	got := apitest.DecodeJSON[shelf.PageList](t, rec)
	list := names(got)
	if len(list) != 3 || list[0] != "a.png" || list[2] != "b.png" || !strings.HasSuffix(list[1], ".png") {
		t.Fatalf("pages = %v, want a new .png page between a.png and b.png", list)
	}
	asset := env.Get(apitest.AssetURL(bookID, sourceID, list[1]))
	apitest.AssertStatus(t, asset, http.StatusOK)
	if !bytes.Equal(asset.Body.Bytes(), image) {
		t.Error("stored page bytes differ from the upload")
	}

	rec = env.RequestTyped(http.MethodPost, insertPageURL(bookID, sourceID, before.MD5Hash, 0), "image/png", bytes.NewReader(image))
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "PAGE_LIST_CONFLICT", pageConflictMessage)

	rec = env.RequestTyped(http.MethodPost, insertPageURL(bookID, sourceID, got.MD5Hash, 9), "image/png", bytes.NewReader(image))
	apitest.AssertErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_PAGE_POSITION", "page position is outside the page list")

	rec = env.RequestTyped(http.MethodPost, insertPageURL(bookID, sourceID, got.MD5Hash, 0), "image/png", strings.NewReader("not an image"))
	apitest.AssertStatus(t, rec, http.StatusBadRequest)

	rec = env.RequestTyped(http.MethodPost, insertPageURL(bookID, sourceID, got.MD5Hash, 0), "image/png", bytes.NewReader(pngHeader(20_000, 20_000)))
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	oversized := append(bytes.Clone(image), make([]byte, 20<<20)...)
	rec = env.RequestTyped(http.MethodPost, insertPageURL(bookID, sourceID, got.MD5Hash, 0), "image/png", bytes.NewReader(oversized))
	apitest.AssertStatus(t, rec, http.StatusRequestEntityTooLarge)

	if after := pagesOf(t, env, bookID, sourceID); len(after.Pages) != 3 || len(after.Unlisted) != 0 {
		t.Fatalf("pages after rejected uploads = %+v, want nothing added", after)
	}
}

func TestAPIDeletePageContract(t *testing.T) {
	env := apitest.New(t)
	bookID, sourceID := newComicForEdits(t, env, "a.png\ngone.png\nb.png\n", "a.png", "b.png", "stray.gif")
	before := pagesOf(t, env, bookID, sourceID)

	rec := env.Delete(deletePageURL(bookID, sourceID, "a.png", before.MD5Hash))
	apitest.AssertStatus(t, rec, http.StatusOK)
	got := apitest.DecodeJSON[shelf.PageList](t, rec)
	if want := []string{"gone.png", "b.png"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("pages = %v, want %v", names(got), want)
	}
	apitest.AssertStatus(t, env.Get(apitest.AssetURL(bookID, sourceID, "a.png")), http.StatusNotFound)
	apitest.AssertStatus(t, env.Get(apitest.AssetURL(bookID, sourceID, "b.png")), http.StatusOK)

	rec = env.Delete(deletePageURL(bookID, sourceID, "gone.png", got.MD5Hash))
	apitest.AssertStatus(t, rec, http.StatusOK)
	got = apitest.DecodeJSON[shelf.PageList](t, rec)

	rec = env.Delete(deletePageURL(bookID, sourceID, "stray.gif", got.MD5Hash))
	apitest.AssertStatus(t, rec, http.StatusOK)
	got = apitest.DecodeJSON[shelf.PageList](t, rec)
	if !reflect.DeepEqual(names(got), []string{"b.png"}) || len(got.Unlisted) != 0 {
		t.Fatalf("pages = %+v, want only b.png left", got)
	}

	rec = env.Delete(deletePageURL(bookID, sourceID, "b.png", before.MD5Hash))
	apitest.AssertErrorEnvelope(t, rec, http.StatusConflict, "PAGE_LIST_CONFLICT", pageConflictMessage)
	apitest.AssertStatus(t, env.Get(apitest.AssetURL(bookID, sourceID, "b.png")), http.StatusOK)
}

func TestAPISetPageDirectionContract(t *testing.T) {
	env := apitest.New(t)
	bookID, sourceID := newComicForEdits(t, env, "a.png\n", "a.png")
	directionURL := apitest.SourceURL(bookID, sourceID, "page-direction")

	rec := env.RequestTyped(http.MethodPut, directionURL, "application/json", strings.NewReader(`{"page_direction":"rtl"}`))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if got := apitest.DecodeJSON[map[string]any](t, rec); got["page_direction"] != "rtl" {
		t.Fatalf("source = %#v, want page_direction rtl", got)
	}
	if got := apitest.GetJSON[map[string]any](t, env, apitest.SourceURL(bookID, sourceID)); got["page_direction"] != "rtl" {
		t.Fatalf("reloaded source = %#v, want page_direction rtl", got)
	}

	rec = env.RequestTyped(http.MethodPut, directionURL, "application/json", strings.NewReader(`{"page_direction":"ltr"}`))
	apitest.AssertStatus(t, rec, http.StatusOK)
	if _, ok := apitest.DecodeJSON[map[string]any](t, rec)["page_direction"]; ok {
		t.Fatal("ltr still reports a page_direction key")
	}

	rec = env.RequestTyped(http.MethodPut, directionURL, "application/json", strings.NewReader(`{"page_direction":"ttb"}`))
	apitest.AssertErrorEnvelope(t, rec, http.StatusBadRequest, "INVALID_PAGE_DIRECTION",
		`page_direction must be "ltr" or "rtl", and "rtl" only on an images source`)
}
