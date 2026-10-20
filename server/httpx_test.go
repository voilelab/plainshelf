package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveShelfRejectsMissingShelfID(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	shelfData, ok := app.handlers.core.resolveShelf(rec, httptest.NewRequest(http.MethodGet, "/api/shelves", nil))

	if ok || shelfData != nil {
		t.Fatalf("resolveShelf = (%v, %v), want (nil, false)", shelfData, ok)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "invalid shelf_id" {
		t.Fatalf("body = %q, want %q", got, "invalid shelf_id")
	}
}

func TestDecodeJSONBody(t *testing.T) {
	type request struct {
		Folder string `json:"folder"`
	}

	const (
		accept = http.StatusOK
		reject = http.StatusBadRequest
	)

	tests := []struct {
		name         string
		body         string
		wantRequired int
		wantOptional int
		wantFolder   string
	}{
		{"an object", `{"folder":"fiction"}`, accept, accept, "fiction"},
		{"trailing whitespace", "{\"folder\":\"fiction\"}\n ", accept, accept, "fiction"},
		{"a JSON null", `null`, accept, accept, ""},

		{"no body", ``, reject, accept, ""},
		{"whitespace only", "  \n\t", reject, accept, ""},
		{"truncated after the name", `{"folder":`, reject, reject, ""},
		{"truncated after the value", `{"folder":"fiction"`, reject, reject, ""},

		{"an unknown member", `{"folder":"fiction","nope":1}`, reject, reject, ""},
		{"a second document", `{"folder":"a"}{"folder":"b"}`, reject, reject, ""},
		{"a non-object", `[1,2]`, reject, reject, ""},
		{"not JSON at all", `@@@`, reject, reject, ""},

		{"a differently cased name", `{"Folder":"fiction"}`, reject, reject, ""},
		{"a repeated member", `{"folder":"a","folder":"b"}`, reject, reject, ""},
		{"invalid UTF-8", "{\"folder\":\"bad \xff\"}", reject, reject, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, variant := range []struct {
				label  string
				decode func(http.ResponseWriter, *http.Request, any) bool
				want   int
			}{
				{"required", decodeStrictJSON, tc.wantRequired},
				{"optional", decodeOptionalStrictJSON, tc.wantOptional},
			} {
				rec := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))

				var got request
				ok := variant.decode(rec, r, &got)

				if ok != (variant.want == accept) {
					t.Errorf("%s: decoded = %v, want %v (body: %s)", variant.label, ok, variant.want == accept, rec.Body.String())
					continue
				}
				if !ok {
					if rec.Code != variant.want {
						t.Errorf("%s: status = %d, want %d", variant.label, rec.Code, variant.want)
					}
					continue
				}
				if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
					t.Errorf("%s: helper wrote status %d body %q, want it to leave the response alone", variant.label, rec.Code, rec.Body.String())
				}
				if got.Folder != tc.wantFolder {
					t.Errorf("%s: folder = %q, want %q", variant.label, got.Folder, tc.wantFolder)
				}
			}
		})
	}
}

func TestDecodeJSONBodyReportsAnOversizedBody(t *testing.T) {
	body := `{"folder":"` + strings.Repeat("x", 4096) + `"}`

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Body = http.MaxBytesReader(rec, r.Body, 64)

	var got struct {
		Folder string `json:"folder"`
	}
	if decodeStrictJSON(rec, r, &got) {
		t.Fatal("decoded an oversized body, want it refused")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func TestJSONDecodeMessage(t *testing.T) {
	type identifiers struct {
		ISBN string `json:"isbn"`
	}
	type request struct {
		Folder      string      `json:"folder"`
		Star        int         `json:"star"`
		Tags        []string    `json:"tags"`
		Identifiers identifiers `json:"identifiers"`
	}

	tests := []struct {
		name string
		body string
		want string
	}{
		{"an unknown field", `{"nope":1}`, `invalid JSON at "nope"`},
		{"a differently cased field", `{"Folder":"x"}`, `invalid JSON at "Folder"`},
		{"an unknown nested field", `{"identifiers":{"nope":1}}`, `invalid JSON at "identifiers/nope"`},
		{"a repeated field", `{"folder":"a","folder":"b"}`, `invalid JSON at "folder"`},

		{"a mistyped field", `{"star":"five"}`, `invalid JSON at "star"`},
		{"a mistyped array element", `{"tags":[1]}`, `invalid JSON at "tags/0"`},
		{"a mistyped nested field", `{"identifiers":{"isbn":5}}`, `invalid JSON at "identifiers/isbn"`},

		{"a truncated value", `{"folder":`, `invalid JSON at "folder"`},
		{"invalid UTF-8", "{\"folder\":\"bad \xff\"}", `invalid JSON at "folder"`},

		{"an empty body", ``, "invalid JSON"},
		{"a body of the wrong shape", `[1,2]`, "invalid JSON"},
		{"a second document", `{"folder":"a"}{"folder":"b"}`, "invalid JSON"},
		{"not JSON at all", `@@@`, "invalid JSON"},

		{"a field name carrying markup", `{"<script>alert(1)</script>":1}`, "invalid JSON"},
		{"an overlong field name", `{"` + strings.Repeat("a", maxDescribedFieldLength+1) + `":1}`, "invalid JSON"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))

			var got request
			if decodeStrictJSON(rec, r, &got) {
				t.Fatalf("decoded %q, want it refused", tc.body)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if body := strings.TrimSpace(rec.Body.String()); body != tc.want {
				t.Errorf("body = %q, want %q", body, tc.want)
			}
		})
	}
}
