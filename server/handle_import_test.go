package server

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf"
)

func TestValidateImportFileHeader(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		wantErr     bool
	}{
		{
			name:        "txt text plain",
			filename:    "book.txt",
			contentType: "text/plain",
		},
		{
			name:        "txt text plain with charset",
			filename:    "book.TXT",
			contentType: "text/plain; charset=utf-8",
		},
		{
			name:        "reject unsupported extension",
			filename:    "book.cbz",
			contentType: "text/plain",
			wantErr:     true,
		},
		{
			name:        "epub with its own media type",
			filename:    "book.epub",
			contentType: "application/epub+zip",
		},
		{
			name:        "epub sent as a generic zip",
			filename:    "book.EPUB",
			contentType: "application/zip",
		},
		{
			name:        "epub sent as an opaque download",
			filename:    "book.epub",
			contentType: "application/octet-stream",
		},
		{
			name:     "epub with no content type",
			filename: "book.epub",
		},
		{
			name:        "reject epub sent as text",
			filename:    "book.epub",
			contentType: "text/plain",
			wantErr:     true,
		},
		{
			name:        "reject non text content type",
			filename:    "book.txt",
			contentType: "application/octet-stream",
			wantErr:     true,
		},
		{
			name:     "reject missing content type",
			filename: "book.txt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &multipart.FileHeader{
				Filename: tt.filename,
				Header:   make(map[string][]string),
			}
			if tt.contentType != "" {
				header.Header.Set("Content-Type", tt.contentType)
			}

			message, err := validateImportFileHeader(header)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// The message is what the client sees, so it must be present on a
			// rejection and must not carry the util.Errorf function prefix the
			// logged error has.
			if tt.wantErr {
				if message == "" {
					t.Fatal("rejection returned no client message")
				}
				if strings.Contains(message, "plainshelf/server") {
					t.Fatalf("message = %q, must not name internal packages", message)
				}
			} else if message != "" {
				t.Fatalf("message = %q, want empty when accepted", message)
			}
		})
	}
}

func TestIsRequestBodyTooLargeUsesMaxBytesErrorType(t *testing.T) {
	wrapped := errors.Join(errors.New("parse multipart"), &http.MaxBytesError{Limit: maxImportBodySize})
	if !isRequestBodyTooLarge(wrapped) {
		t.Fatal("expected wrapped http.MaxBytesError to be recognized")
	}
	if isRequestBodyTooLarge(errors.New("http: request body too large")) {
		t.Fatal("plain string-compatible error must not be recognized as MaxBytesError")
	}
}

func TestWriteEPUBImportErrorClassifiesFailures(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantStatus   int
		wantBody     string
		forbidInBody string
	}{
		{
			name:       "invalid archive is a client error",
			err:        &epubInputError{cause: errors.New("broken archive")},
			wantStatus: http.StatusBadRequest,
			wantBody:   "broken archive",
		},
		{
			name:         "storage failure is a generic server error",
			err:          errors.New("disk is full at /private/shelf"),
			wantStatus:   http.StatusInternalServerError,
			wantBody:     "failed to import epub",
			forbidInBody: "/private/shelf",
		},
		{
			// An import creates a book, so it fails for the same reasons any
			// other write does and must get the same status for them.
			name:       "a refused folder is a client error",
			err:        util.Errorf("%w", shelf.ErrInvalidFolder),
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid folder name",
		},
	}

	app := newTestApp(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.handlers.imports.writeEPUBImportError(rec,
				httptest.NewRequest(http.MethodPost, "/api/shelves/s/books/import", nil), tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tt.wantBody)
			}
			if tt.forbidInBody != "" && strings.Contains(rec.Body.String(), tt.forbidInBody) {
				t.Errorf("body = %q, must not expose %q", rec.Body.String(), tt.forbidInBody)
			}
		})
	}
}

func TestMultipartDefaultFileContentTypeIsRejected(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "book.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	defer form.RemoveAll()

	files := form.File["file"]
	if len(files) != 1 {
		t.Fatalf("expected one file, got %d", len(files))
	}
	if _, err := validateImportFileHeader(files[0]); err == nil {
		t.Fatal("expected default application/octet-stream upload to be rejected")
	}
}

// TestImportFromLocalPathStripsExtensionFromTitle pins that the desktop local-path
// import derives the book title from the filename with its extension removed, so
// "遮天.txt" becomes "遮天" rather than "遮天.txt" — matching the web-upload path,
// whose frontend already sends a de-extensioned title.
func TestImportFromLocalPathStripsExtensionFromTitle(t *testing.T) {
	app := newTestApp(t)

	srcPath := filepath.Join(t.TempDir(), "遮天.txt")
	if err := os.WriteFile(srcPath, []byte("第一章\n\n內文。\n"), 0o600); err != nil {
		t.Fatalf("write source book: %v", err)
	}

	book, err := app.ImportFromLocalPath("default_shelf", srcPath, nil)
	if err != nil {
		t.Fatalf("ImportFromLocalPath returned error: %v", err)
	}
	if got := book.GetMeta().Title; got != "遮天" {
		t.Fatalf("title = %q, want 遮天", got)
	}
}

func TestValidateLocalImportPath(t *testing.T) {
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "book.txt")
	if err := os.WriteFile(validPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	epubPath := filepath.Join(tmpDir, "book.epub")
	if err := os.WriteFile(epubPath, []byte("PK"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid txt file", input: validPath},
		{name: "trimmed path", input: " " + validPath + " "},
		{name: "valid epub file", input: epubPath},
		{name: "reject empty", input: "", wantErr: true},
		{name: "reject unsupported extension", input: filepath.Join(tmpDir, "book.cbz"), wantErr: true},
		{name: "reject missing file", input: filepath.Join(tmpDir, "absent.epub"), wantErr: true},
		{name: "reject directory", input: tmpDir, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateLocalImportPath(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
