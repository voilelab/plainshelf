package readerapi_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

type blockingRecorder struct {
	*httptest.ResponseRecorder

	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func newBlockingRecorder() *blockingRecorder {
	return &blockingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		started:          make(chan struct{}),
		release:          make(chan struct{}),
	}
}

func (r *blockingRecorder) Write(data []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
		<-r.release
	})
	return r.ResponseRecorder.Write(data)
}

func TestOpenAnotherBookWaitsForReads(t *testing.T) {
	library := newOpenLibrary(t)
	handler := readerapi.NewHandler(library, "test")

	recorder := newBlockingRecorder()
	served := make(chan struct{})
	go func() {
		defer close(served)
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, bookPath(testBookID)+"/content", nil))
	}()

	<-recorder.started

	opened := make(chan struct{})
	go func() {
		defer close(opened)
		if _, err := library.Open(writeBookPackage(t, "Emma", "emma5678")); err != nil {
			t.Errorf("Open: %v", err)
		}
	}()

	select {
	case <-opened:
		t.Fatal("opening another book completed while a read was still in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(recorder.release)
	<-served
	<-opened

	if recorder.Code != http.StatusOK {
		t.Errorf("in-flight request status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != testContent {
		t.Errorf("in-flight request body = %q, want the whole source %q", body, testContent)
	}
	if library.BookID() != "emma5678" {
		t.Errorf("open book = %q, want the newly opened one", library.BookID())
	}
}
