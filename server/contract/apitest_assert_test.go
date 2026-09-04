package contract_test

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const PlainTextContentType = "text/plain; charset=utf-8"

const jsonContentType = "application/json; charset=utf-8"

// wailsLikeRecorder records a response the way the Wails asset server does:
// with no implied status, so a handler that never calls WriteHeader is visible
// as such instead of being reported as 200.
type wailsLikeRecorder struct {
	header      http.Header
	body        bytes.Buffer
	code        int
	wroteHeader bool
}

func newWailsLikeRecorder() *wailsLikeRecorder {
	return &wailsLikeRecorder{
		header: http.Header{},
		code:   http.StatusNotImplemented,
	}
}

func (rec *wailsLikeRecorder) Header() http.Header {
	return rec.header
}

func (rec *wailsLikeRecorder) Write(buf []byte) (int, error) {
	if !rec.wroteHeader {
		rec.WriteHeader(http.StatusOK)
	}
	return rec.body.Write(buf)
}

func (rec *wailsLikeRecorder) WriteHeader(code int) {
	if rec.wroteHeader {
		return
	}
	rec.wroteHeader = true
	rec.code = code
}

func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, want, rec.Body.String())
	}
}

func AssertContentType(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != want {
		t.Fatalf("Content-Type = %q, want %s", got, want)
	}
}

func AssertJSONContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	AssertContentType(t, rec, jsonContentType)
}

func assertWailsLikeStatus(t *testing.T, rec *wailsLikeRecorder, want int) {
	t.Helper()
	if rec.code != want {
		t.Fatalf("status = %d, want %d, body: %s", rec.code, want, rec.body.String())
	}
}

// AssertEmptyTextResponse asserts a streamed text response carried the plain
// text content type and an empty body, which is what an empty file must look
// like rather than an error.
func AssertEmptyTextResponse(t *testing.T, what string, rec *wailsLikeRecorder) {
	t.Helper()

	assertWailsLikeStatus(t, rec, http.StatusOK)
	if got := rec.header.Get("Content-Type"); got != PlainTextContentType {
		t.Fatalf("%s Content-Type = %q, want %s", what, got, PlainTextContentType)
	}
	if got := rec.body.String(); got != "" {
		t.Fatalf("%s content = %q, want empty", what, got)
	}
}

func DecodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
	return out
}

// GetJSON reads a JSON endpoint that must answer 200 and decodes its body.
// Tests that also pin the response's content type issue the request themselves.
func GetJSON[T any](t *testing.T, env *Env, url string) T {
	t.Helper()

	rec := env.Get(url)
	AssertStatus(t, rec, http.StatusOK)
	return DecodeJSON[T](t, rec)
}

// AssertMutationGated asserts a mutating route stays behind both write gates:
// the request is refused without the local token, and refused again in read-only
// mode even with it. Every mutating endpoint shares this contract, so the pair
// lives here rather than being restated per endpoint.
func AssertMutationGated(t *testing.T, env *Env, method, url string, body []byte) {
	t.Helper()

	newBody := func() io.Reader {
		if body == nil {
			return nil
		}
		return bytes.NewReader(body)
	}

	// DoRaw omits the token Do() would attach.
	rec := env.DoRaw(httptest.NewRequest(method, url, newBody()))
	AssertStatus(t, rec, http.StatusUnauthorized)

	// Read-only mode is restored right away, so each gate is exercised on its own
	// and a caller can keep making requests afterwards.
	previous := env.App.Conf().ReadOnly
	env.App.Conf().ReadOnly = true
	rec = env.Do(httptest.NewRequest(method, url, newBody()))
	env.App.Conf().ReadOnly = previous
	AssertStatus(t, rec, http.StatusForbidden)
}

// ErrorEnvelope is the body shape every writeErr refusal carries. Tests
// assert against the decoded envelope rather than the raw text so a message
// reworded inside it does not read as a change of error identity.
type ErrorEnvelope struct {
	Error struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Incident string `json:"incident"`
	} `json:"error"`
}

// AssertErrorEnvelope pins a refusal's status, content type, code and message
// together, because the three travel as one contract: a client that switches on
// the code needs the JSON body to have arrived at all.
func AssertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()

	AssertStatus(t, rec, status)
	AssertJSONContentType(t, rec)

	got := DecodeJSON[ErrorEnvelope](t, rec)
	if got.Error.Code != code {
		t.Fatalf("error code = %q, want %q (body: %s)", got.Error.Code, code, rec.Body.String())
	}
	if got.Error.Message != message {
		t.Fatalf("error message = %q, want %q", got.Error.Message, message)
	}
}
