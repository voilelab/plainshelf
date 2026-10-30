package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/voilelab/plainshelf/internal/logutil"
)

func TestWriteJSONCommitsAfterMarshal(t *testing.T) {
	core := &apiCore{Logger: &logutil.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	for _, tc := range []struct {
		name       string
		value      any
		wantStatus int
		wantType   string
		wantBody   string
	}{
		{"success", map[string]string{"value": "ok"}, http.StatusCreated, "application/json; charset=utf-8", "{\"value\":\"ok\"}\n"},
		{"marshal failure", make(chan int), http.StatusInternalServerError, "text/plain; charset=utf-8", "failed to encode response\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			core.writeJSON(rec, http.StatusCreated, tc.value)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != tc.wantType {
				t.Errorf("content type = %q, want %q", got, tc.wantType)
			}
			if got := rec.Body.String(); got != tc.wantBody {
				t.Errorf("body = %q, want %q", got, tc.wantBody)
			}
		})
	}
}
