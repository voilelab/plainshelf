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

func TestCloseBookWaitsForReads(t *testing.T) {
	library := newOpenLibrary(t)
	handler := readerapi.NewHandler(library, "test")

	recorder := newBlockingRecorder()
	served := make(chan struct{})
	go func() {
		defer close(served)
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, bookPath(testBookID)+"/content", nil))
	}()

	<-recorder.started

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		if err := library.CloseBook(testBookID); err != nil {
			t.Errorf("CloseBook: %v", err)
		}
	}()

	select {
	case <-closed:
		t.Fatal("closing the book completed while a read was still in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(recorder.release)
	<-served
	<-closed

	if recorder.Code != http.StatusOK {
		t.Errorf("in-flight request status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != testContent {
		t.Errorf("in-flight request body = %q, want the whole source %q", body, testContent)
	}
	if library.Has(testBookID) {
		t.Error("book still held after CloseBook")
	}
}
