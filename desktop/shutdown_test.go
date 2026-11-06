package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingclose"
	"github.com/voilelab/plainshelf/internal/readingprogress"
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

func diskOffset(t *testing.T, store *readingprogress.Store, shelfID, bookID string) int64 {
	t.Helper()
	doc, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	return doc.Shelves[shelfID][bookID].Offset
}

func newShutdownApp(t *testing.T, timeout time.Duration) (*DesktopApp, *readingprogress.Store, string) {
	t.Helper()
	env := newLANShareEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	app := &DesktopApp{
		native:         &fakeNative{},
		app:            env.serverApp,
		lanShare:       env.share,
		progressStager: readingclose.NewStager(store, timeout),
	}
	addr := strings.TrimSuffix(strings.TrimPrefix(st.URL, "http://"), "/dav/")
	return app, store, addr
}

func TestShutdownWritesProgressThenStopsLAN(t *testing.T) {
	app, store, addr := newShutdownApp(t, time.Second)
	app.StageReadingProgress("a", "book-a", 420, 1000)

	app.closeMainWindow()

	if got := diskOffset(t, store, "a", "book-a"); got != 420 {
		t.Fatalf("persisted offset = %d, want 420", got)
	}
	if lanDialable(addr) {
		t.Fatalf("LAN share still accepts connections on %s after shutdown", addr)
	}
}

func TestShutdownTwiceIsNoop(t *testing.T) {
	app, store, _ := newShutdownApp(t, time.Second)
	app.StageReadingProgress("a", "book-a", 420, 1000)
	app.closeMainWindow()

	app.StageReadingProgress("a", "book-a", 999, 2000)
	app.Shutdown()

	if got := diskOffset(t, store, "a", "book-a"); got != 420 {
		t.Fatalf("offset after second shutdown = %d, want 420 from the first", got)
	}
}

func TestShutdownWaitsOnProgressThenCloses(t *testing.T) {
	app, store, addr := newShutdownApp(t, readingclose.DefaultTimeout)
	app.StageReadingProgress("a", "book-a", 420, 1000)
	release := holdProgressLock(t, store)

	done := make(chan struct{})
	go func() {
		app.Shutdown()
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("shutdown returned while the progress write was still blocked")
	default:
	}
	if !lanDialable(addr) {
		t.Fatal("LAN share stopped before the progress write finished")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not continue after the progress write timed out")
	}
	if lanDialable(addr) {
		t.Fatalf("LAN share still accepts connections on %s after a timed-out write", addr)
	}

	release()
	deadline := time.Now().Add(5 * time.Second)
	for diskOffset(t, store, "a", "book-a") != 420 {
		if time.Now().After(deadline) {
			t.Fatal("late progress write never landed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
