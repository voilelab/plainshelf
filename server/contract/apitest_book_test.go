package contract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// The URL builders below assemble the routes the tests share. They keep the
// repeated /api/shelves/<id> prefix in one place while leaving each route's own
// shape spelled out at the call site, since that shape is what is under test.

func shelfIDURL(shelfID string, elem ...string) string {
	return strings.Join(append([]string{"/api/shelves", shelfID}, elem...), "/")
}

// shelfURL addresses the single shelf the contract tests configure.
func shelfURL(elem ...string) string {
	return shelfIDURL(defaultShelfID, elem...)
}

func booksURL() string {
	return shelfURL("books")
}

func bookURL(bookID string, elem ...string) string {
	return shelfURL(append([]string{"books", bookID}, elem...)...)
}

func sourceURL(bookID, sourceID string, elem ...string) string {
	return bookURL(bookID, append([]string{"sources", sourceID}, elem...)...)
}

// assetURL addresses one file under a source's assets/ directory. The name is
// used verbatim so a test can address an already-escaped or otherwise unusual
// name and see what the route does with it.
func assetURL(bookID, sourceID, name string) string {
	return sourceURL(bookID, sourceID, "assets") + "/" + name
}

// formUpload is a multipart request body: text fields in the given order,
// followed by at most one file part. Every multipart request in these tests goes
// through it, so the writer boilerplate exists once.
type formUpload struct {
	fields [][2]string

	// fileField is the form key of the file part. The part is omitted entirely
	// when filename is empty, which is how "a request with no file" is expressed.
	fileField   string
	filename    string
	contentType string
	content     string
}

// request builds the multipart request, including the Content-Type header
// carrying the generated boundary.
func (up formUpload) request(t *testing.T, method, url string) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, field := range up.fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatalf("WriteField %s: %v", field[0], err)
		}
	}

	if up.filename != "" {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name=%q; filename=%q`, up.fileField, up.filename))
		if up.contentType != "" {
			header.Set("Content-Type", up.contentType)
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := io.Copy(part, strings.NewReader(up.content)); err != nil {
			t.Fatalf("write multipart file: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// setFields drops the fields whose value is empty, so a caller can leave "title",
// "folder" or "strategy" out of the form by passing "".
func setFields(fields ...[2]string) [][2]string {
	set := make([][2]string, 0, len(fields))
	for _, field := range fields {
		if field[1] != "" {
			set = append(set, field)
		}
	}
	return set
}

// bookUpload describes an upload of the import endpoint's "file" part.
func bookUpload(filename, contentType, content string, fields ...[2]string) formUpload {
	return formUpload{
		fields:      setFields(fields...),
		fileField:   "file",
		filename:    filename,
		contentType: contentType,
		content:     content,
	}
}

// postBookImport sends an upload to the import endpoint and returns the raw
// recorder, so a caller can assert either the created book or a rejection.
func postBookImport(t *testing.T, env *apiTestEnv, up formUpload) *httptest.ResponseRecorder {
	t.Helper()
	return env.do(up.request(t, http.MethodPost, shelfURL("books", "import")))
}

// importBook uploads a book, asserts it was created, and returns it.
func importBook(t *testing.T, env *apiTestEnv, up formUpload) server.Book {
	t.Helper()

	rec := postBookImport(t, env, up)
	assertStatus(t, rec, http.StatusCreated)
	assertJSONContentType(t, rec)
	return decodeJSON[server.Book](t, rec)
}

// importTextBook imports a plain-text book. The uploaded bytes carry a BOM and a
// non-ASCII tail, so every caller also exercises the decoding path. An empty
// title or folder is left out of the form rather than sent empty.
func importTextBook(t *testing.T, env *apiTestEnv, title, folder, filename, body string) server.Book {
	t.Helper()

	return importBook(t, env, bookUpload(filename, plainTextContentType, "\ufeff"+body+"\n世界",
		[2]string{"title", title}, [2]string{"folder", folder}))
}

// importFileBook uploads a book file with an explicit filename and content type.
// It complements importTextBook by letting callers exercise the other
// browser-supplied content types, such as those sent for a Markdown upload.
func importFileBook(t *testing.T, env *apiTestEnv, filename, contentType, body string) server.Book {
	t.Helper()

	return importBook(t, env, bookUpload(filename, contentType, body))
}

// epubUpload describes an EPUB import, optionally carrying a conversion strategy.
func epubUpload(filename, archive, strategy string) formUpload {
	return bookUpload(filename, "application/epub+zip", archive, [2]string{"strategy", strategy})
}

// importEPUBWithStrategy uploads an EPUB with an explicit strategy field and
// asserts the import succeeds.
func importEPUBWithStrategy(t *testing.T, env *apiTestEnv, filename, archive, strategy string) server.Book {
	t.Helper()

	return importBook(t, env, epubUpload(filename, archive, strategy))
}

// importEPUBExpectingStatus uploads an EPUB that is expected to be refused.
func importEPUBExpectingStatus(t *testing.T, env *apiTestEnv, filename, archive, strategy string, want int) {
	t.Helper()

	rec := postBookImport(t, env, epubUpload(filename, archive, strategy))
	assertStatus(t, rec, want)
}

// bookMetaPath locates the on-disk book.json for the given book ID by walking
// the shelf root, so tests do not have to reproduce the folder-naming scheme.
func (env *apiTestEnv) bookMetaPath(t *testing.T, bookID string) string {
	t.Helper()

	var found string
	err := filepath.WalkDir(env.libRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != shelf.BookMetaFile || found != "" {
			return err
		}
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		var meta shelf.BookMeta
		if json.Unmarshal(raw, &meta) == nil && meta.ID == bookID {
			found = p
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk lib root: %v", err)
	}
	if found == "" {
		t.Fatalf("could not find book.json for book %s under %s", bookID, env.libRoot)
	}
	return found
}

// bumpBookSchemaVersion rewrites a book's book.json with a schema version this
// build does not support, reproducing a book written by a newer PlainShelf. It
// returns the bytes now on disk so a caller can prove a refused write left them
// alone.
func bumpBookSchemaVersion(t *testing.T, env *apiTestEnv, bookID string, extra map[string]any) (metaPath string, onDisk []byte) {
	t.Helper()

	metaPath = env.bookMetaPath(t, bookID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	meta["schema_version"] = shelf.BookMetaSchemaVersion + 1
	maps.Copy(meta, extra)

	bumped, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal book.json: %v", err)
	}
	if err := os.WriteFile(metaPath, bumped, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}
	return metaPath, bumped
}

// currentSourceID returns the source the imported book is reading from.
func (env *apiTestEnv) currentSourceID(t *testing.T, bookID string) string {
	t.Helper()

	metas := getJSON[[]shelf.SourceMeta](t, env, bookURL(bookID, "sources"))
	if len(metas) == 0 {
		t.Fatalf("book %s has no sources", bookID)
	}
	return metas[0].ID
}

// writeSourceAsset drops a file into a source's assets/ directory, which is how
// an illustration gets there today: the API serves assets but cannot store them.
func (env *apiTestEnv) writeSourceAsset(t *testing.T, bookID, sourceID, name string, data []byte) {
	t.Helper()

	bookDir := filepath.Dir(env.bookMetaPath(t, bookID))
	assetDir := filepath.Join(bookDir, shelf.SourcesFolder, sourceID, shelf.SourceAssetsFolder)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", assetDir, err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, name), data, 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}
