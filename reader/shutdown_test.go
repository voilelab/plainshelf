package main

import (
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
	"github.com/voilelab/plainshelf/reader/readerapi"
)

func holdProgressLock(t *testing.T, store *readingprogress.Store) (release func()) {
	t.Helper()
	locked, unlock, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, _ = store.Mutate(func(doc readingprogress.Document) readingprogress.Document {
			close(locked)
			<-unlock
			return doc
		})
	}()
	<-locked
	return func() {
		close(unlock)
		<-done
	}
}

func newShutdownReader(t *testing.T, timeout time.Duration) *ReaderApp {
	t.Helper()
	app, _ := newTestApp(t)
	app.closeTimeout = timeout
	launchBook(t, app, "")
	app.StageReadingProgress(readerapi.ShelfID, testBookID, 4321, 1000)
	return app
}

func stagedOffset(t *testing.T, app *ReaderApp) int64 {
	t.Helper()
	return diskDoc(t, app).Shelves[readerapi.ShelfID][testBookID].Offset
}

func TestReaderShutdownWritesThenCloses(t *testing.T) {
	app := newShutdownReader(t, time.Second)

	app.Shutdown(t.Context())

	if got := stagedOffset(t, app); got != 4321 {
		t.Fatalf("persisted offset = %d, want 4321", got)
	}
	if app.library.Has(testBookID) {
		t.Fatal("book still open after shutdown")
	}
}

func TestReaderShutdownTwiceIsNoop(t *testing.T) {
	app := newShutdownReader(t, time.Second)
	app.Shutdown(t.Context())

	app.StageReadingProgress(readerapi.ShelfID, testBookID, 9999, 2000)
	app.Shutdown(t.Context())

	if got := stagedOffset(t, app); got != 4321 {
		t.Fatalf("offset after second shutdown = %d, want 4321 from the first", got)
	}
}

func TestReaderShutdownWaitsThenCloses(t *testing.T) {
	app := newShutdownReader(t, readingclose.DefaultTimeout)
	release := holdProgressLock(t, app.progressStore)

	done := make(chan struct{})
	go func() {
		app.Shutdown(t.Context())
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("shutdown returned while the progress write was still blocked")
	default:
	}
	if !app.library.Has(testBookID) {
		t.Fatal("library closed before the progress write finished")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not continue after the progress write timed out")
	}
	if app.library.Has(testBookID) {
		t.Fatal("book still open after a timed-out write")
	}

	release()
	deadline := time.Now().Add(5 * time.Second)
	for stagedOffset(t, app) != 4321 {
		if time.Now().After(deadline) {
			t.Fatal("late progress write never landed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
