package apitest

import (
	"bytes"
	"encoding/json/v2"
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

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/server"
	"github.com/voilelab/plainshelf/shelf"
)

// The URL builders below assemble the routes the tests share. They keep the
// repeated /api/shelves/<id> prefix in one place while leaving each route's own
// shape spelled out at the call site, since that shape is what is under test.

// FormUpload is a multipart request body: text fields in the given order,
// followed by at most one file part. Every multipart request in these tests goes
// through it, so the writer boilerplate exists once.
type FormUpload struct {
	Fields [][2]string

	// FileField is the form key of the file part. The part is omitted entirely
	// when Filename is empty, which is how "a request with no file" is expressed.
	FileField   string
	Filename    string
	ContentType string
	Content     string
}

// Request builds the multipart request, including the Content-Type header
// carrying the generated boundary.
func (up FormUpload) Request(t *testing.T, method, url string) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, field := range up.Fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatalf("WriteField %s: %v", field[0], err)
		}
	}

	if up.Filename != "" {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name=%q; filename=%q`, up.FileField, up.Filename))
		if up.ContentType != "" {
			header.Set("Content-Type", up.ContentType)
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := io.Copy(part, strings.NewReader(up.Content)); err != nil {
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

// BookUpload describes an upload of the import endpoint's "file" part.
func BookUpload(filename, contentType, content string, fields ...[2]string) FormUpload {
	return FormUpload{
		Fields:      setFields(fields...),
		FileField:   "file",
		Filename:    filename,
		ContentType: contentType,
		Content:     content,
	}
}

// PostBookImport sends an upload to the import endpoint and returns the raw
// recorder, so a caller can assert either the created book or a rejection.
func PostBookImport(t *testing.T, env *Env, up FormUpload) *httptest.ResponseRecorder {
	t.Helper()
	return env.Do(up.Request(t, http.MethodPost, ShelfURL("books", "import")))
}

// importBook uploads a book, asserts it was created, and returns it.
func importBook(t *testing.T, env *Env, up FormUpload) server.Book {
	t.Helper()

	rec := PostBookImport(t, env, up)
	AssertStatus(t, rec, http.StatusCreated)
	AssertJSONContentType(t, rec)
	return DecodeJSON[server.Book](t, rec)
}

// ImportTextBook imports a plain-text book. The uploaded bytes carry a BOM and a
// non-ASCII tail, so every caller also exercises the decoding path. An empty
// title or folder is left out of the form rather than sent empty.
func ImportTextBook(t *testing.T, env *Env, title, folder, filename, body string) server.Book {
	t.Helper()

	return importBook(t, env, BookUpload(filename, PlainTextContentType, "\ufeff"+body+"\n世界",
		[2]string{"title", title}, [2]string{"folder", folder}))
}

// ListedBookIDs is the IDs the books endpoint reports, which is what a test
// asserting who is and is not served compares against.
func ListedBookIDs(t *testing.T, env *Env) []string {
	t.Helper()

	ids := []string{}
	for _, book := range GetJSON[[]server.Book](t, env, BooksURL()) {
		ids = append(ids, book.Meta.ID)
	}
	return ids
}

// ImportFileBook uploads a book file with an explicit filename and content type.
// It complements ImportTextBook by letting callers exercise the other
// browser-supplied content types, such as those sent for a Markdown upload.
func ImportFileBook(t *testing.T, env *Env, filename, contentType, body string) server.Book {
	t.Helper()

	return importBook(t, env, BookUpload(filename, contentType, body))
}

// epubUpload describes an EPUB import, optionally carrying a conversion strategy.
func epubUpload(filename, archive, strategy string) FormUpload {
	return BookUpload(filename, "application/epub+zip", archive, [2]string{"strategy", strategy})
}

// ImportEPUBWithStrategy uploads an EPUB with an explicit strategy field and
// asserts the import succeeds.
func ImportEPUBWithStrategy(t *testing.T, env *Env, filename, archive, strategy string) server.Book {
	t.Helper()

	return importBook(t, env, epubUpload(filename, archive, strategy))
}

// ImportEPUBExpectingStatus uploads an EPUB that is expected to be refused.
func ImportEPUBExpectingStatus(t *testing.T, env *Env, filename, archive, strategy string, want int) {
	t.Helper()

	rec := PostBookImport(t, env, epubUpload(filename, archive, strategy))
	AssertStatus(t, rec, want)
}

// BookMetaPath locates the on-disk book.json for the given book ID by walking
// the shelf root, so tests do not have to reproduce the folder-naming scheme.
func (env *Env) BookMetaPath(t *testing.T, bookID string) string {
	t.Helper()

	var found string
	err := filepath.WalkDir(env.LibRoot, func(p string, d os.DirEntry, err error) error {
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
		t.Fatalf("could not find book.json for book %s under %s", bookID, env.LibRoot)
	}
	return found
}

// EditBookMetaFile merges fields into a book's book.json on disk, for a fact no
// API can write - a schema version from a newer build, or the nsfw mark, which
// has no endpoint yet. It returns the path and the bytes now there, so a caller
// can prove a refused write left them alone; a caller that needs the shelf to
// read the edit back rescans afterwards.
func EditBookMetaFile(t *testing.T, env *Env, bookID string, edit map[string]any) (metaPath string, onDisk []byte) {
	t.Helper()

	metaPath = env.BookMetaPath(t, bookID)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read book.json: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal book.json: %v", err)
	}
	maps.Copy(meta, edit)

	edited, err := json.Marshal(meta, jsonopt.Disk())
	if err != nil {
		t.Fatalf("marshal book.json: %v", err)
	}
	if err := os.WriteFile(metaPath, edited, 0o644); err != nil {
		t.Fatalf("write book.json: %v", err)
	}
	return metaPath, edited
}

// BumpBookSchemaVersion rewrites a book's book.json with a schema version this
// build does not support, reproducing a book written by a newer PlainShelf.
func BumpBookSchemaVersion(t *testing.T, env *Env, bookID string, extra map[string]any) (metaPath string, onDisk []byte) {
	t.Helper()

	edit := map[string]any{"schema_version": shelf.BookMetaSchemaVersion + 1}
	maps.Copy(edit, extra)
	return EditBookMetaFile(t, env, bookID, edit)
}

// CurrentSourceID returns the source the imported book is reading from.
func (env *Env) CurrentSourceID(t *testing.T, bookID string) string {
	t.Helper()

	metas := GetJSON[[]shelf.SourceMeta](t, env, BookURL(bookID, "sources"))
	if len(metas) == 0 {
		t.Fatalf("book %s has no sources", bookID)
	}
	return metas[0].ID
}

// WriteSourceAsset drops a file into a source's assets/ directory, which is how
// an illustration gets there today: the API serves assets but cannot store them.
func (env *Env) WriteSourceAsset(t *testing.T, bookID, sourceID, name string, data []byte) {
	t.Helper()

	bookDir := filepath.Dir(env.BookMetaPath(t, bookID))
	assetDir := filepath.Join(bookDir, shelf.SourcesFolder, sourceID, shelf.SourceAssetsFolder)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", assetDir, err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, name), data, 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}

// PatchBook sends a PATCH to a book and asserts the status it must answer with.
// The body is JSON in every case, so a caller reads as the field-level contract
// it is pinning.
func PatchBook(t *testing.T, env *Env, bookID, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()

	rec := env.Patch(BookURL(bookID), strings.NewReader(body))
	AssertStatus(t, rec, wantStatus)
	return rec
}
