package server

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"testing"
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
			name:        "reject non txt extension",
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

			err := validateImportFileHeader(header)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
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
	if err := validateImportFileHeader(files[0]); err == nil {
		t.Fatal("expected default application/octet-stream upload to be rejected")
	}
}
