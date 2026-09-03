package contract_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const plainTextContentType = "text/plain; charset=utf-8"

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

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, want, rec.Body.String())
	}
}

func assertContentType(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != want {
		t.Fatalf("Content-Type = %q, want %s", got, want)
	}
}

func assertJSONContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assertContentType(t, rec, jsonContentType)
}

func assertWailsLikeStatus(t *testing.T, rec *wailsLikeRecorder, want int) {
	t.Helper()
	if rec.code != want {
		t.Fatalf("status = %d, want %d, body: %s", rec.code, want, rec.body.String())
	}
}

// assertEmptyTextResponse asserts a streamed text response carried the plain
// text content type and an empty body, which is what an empty file must look
// like rather than an error.
func assertEmptyTextResponse(t *testing.T, what string, rec *wailsLikeRecorder) {
	t.Helper()

	assertWailsLikeStatus(t, rec, http.StatusOK)
	if got := rec.header.Get("Content-Type"); got != plainTextContentType {
		t.Fatalf("%s Content-Type = %q, want %s", what, got, plainTextContentType)
	}
	if got := rec.body.String(); got != "" {
		t.Fatalf("%s content = %q, want empty", what, got)
	}
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
	return out
}

// getJSON reads a JSON endpoint that must answer 200 and decodes its body.
// Tests that also pin the response's content type issue the request themselves.
func getJSON[T any](t *testing.T, env *apiTestEnv, url string) T {
	t.Helper()

	rec := env.get(url)
	assertStatus(t, rec, http.StatusOK)
	return decodeJSON[T](t, rec)
}

// assertMutationGated asserts a mutating route stays behind both write gates:
// the request is refused without the local token, and refused again in read-only
// mode even with it. Every mutating endpoint shares this contract, so the pair
// lives here rather than being restated per endpoint.
func assertMutationGated(t *testing.T, env *apiTestEnv, method, url string, body []byte) {
	t.Helper()

	newBody := func() io.Reader {
		if body == nil {
			return nil
		}
		return bytes.NewReader(body)
	}

	// doRaw omits the token do() would attach.
	rec := env.doRaw(httptest.NewRequest(method, url, newBody()))
	assertStatus(t, rec, http.StatusUnauthorized)

	// Read-only mode is restored right away, so each gate is exercised on its own
	// and a caller can keep making requests afterwards.
	previous := env.app.Conf().ReadOnly
	env.app.Conf().ReadOnly = true
	rec = env.do(httptest.NewRequest(method, url, newBody()))
	env.app.Conf().ReadOnly = previous
	assertStatus(t, rec, http.StatusForbidden)
}

// apiErrorEnvelope is the body shape every writeErr refusal carries. Tests
// assert against the decoded envelope rather than the raw text so a message
// reworded inside it does not read as a change of error identity.
type apiErrorEnvelope struct {
	Error struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Incident string `json:"incident"`
	} `json:"error"`
}

// assertErrorEnvelope pins a refusal's status, content type, code and message
// together, because the three travel as one contract: a client that switches on
// the code needs the JSON body to have arrived at all.
func assertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()

	assertStatus(t, rec, status)
	assertJSONContentType(t, rec)

	got := decodeJSON[apiErrorEnvelope](t, rec)
	if got.Error.Code != code {
		t.Fatalf("error code = %q, want %q (body: %s)", got.Error.Code, code, rec.Body.String())
	}
	if got.Error.Message != message {
		t.Fatalf("error message = %q, want %q", got.Error.Message, message)
	}
}
