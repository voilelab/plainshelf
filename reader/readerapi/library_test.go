package readerapi_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/reader/readerapi"
)

// blockingRecorder holds a response open at its first write, which is where a
// handler has already resolved its book and is reading through the package.
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

// Opening another book must not pull the directory handle out from under a
// request that is still reading the current one.
//
// The failure this pins is not theoretical: a handler resolves the book, then
// opens its source or an asset through the same os.Root, and those are separate
// reads. Closing the package between them ends the response halfway with an
// error the user cannot act on.
func TestOpeningAnotherBookWaitsForInFlightReads(t *testing.T) {
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
