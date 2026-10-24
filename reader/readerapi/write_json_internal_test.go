package readerapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONAnswersAnUnencodableValueWithACleanError(t *testing.T) {
	h := &handlers{lib: NewLibrary(nil)}
	rec := httptest.NewRecorder()

	h.writeJSON(rec, struct {
		Text   string   `json:"text"`
		Broken chan int `json:"broken"`
	}{Text: strings.Repeat("x", 1<<20), Broken: make(chan int)})

	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "failed to encode response\n" {
		t.Fatalf("response = %d %q, want 500 and only the error text", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the error's text/plain", got)
	}
}
