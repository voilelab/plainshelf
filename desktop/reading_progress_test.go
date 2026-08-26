package main

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/readingprogress"
)

func seedStore(t *testing.T, store *readingprogress.Store, mutate func(readingprogress.Document) readingprogress.Document) {
	t.Helper()
	if _, err := store.Mutate(mutate); err != nil {
		t.Fatalf("seed store: %v", err)
	}
}

func readerOffset(t *testing.T, store *readingprogress.Store, shelfKey, bookID string) int64 {
	t.Helper()
	doc, _, err := store.Read()
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	return doc.Shelves[shelfKey][bookID].Offset
}

// A reader progress entry is projected onto the real shelf that holds the book,
// and the projection is persisted so a second read is a no-op.
func TestProjectStoredReaderProgress_FoldsAndPersists(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{"book-a": {Offset: 120, At: 1000}}
		return out
	})

	resolve := func(bookID string) (string, bool) {
		if bookID == "book-a" {
			return "real-shelf", true
		}
		return "", false
	}

	text, err := projectStoredReaderProgress(store, resolve)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	got := readingprogress.Parse(text)
	if v := got.Shelves["real-shelf"]["book-a"].Offset; v != 120 {
		t.Fatalf("returned real-shelf offset = %d, want 120", v)
	}
	// Persisted, and the reader namespace is kept.
	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 120 {
		t.Fatalf("persisted real-shelf offset = %d, want 120", v)
	}
	if v := readerOffset(t, store, readingprogress.ReaderShelfID, "book-a"); v != 120 {
		t.Fatalf("reader entry offset = %d, want 120 (must be kept)", v)
	}
}

// An unresolved book (no shelf holds it) is not projected and stays under the
// reader namespace; the read returns the stored document untouched.
func TestProjectStoredReaderProgress_UnresolvedStays(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{"book-loose": {Offset: 30, At: 1000}}
		return out
	})

	unresolved := func(string) (string, bool) { return "", false }
	if _, err := projectStoredReaderProgress(store, unresolved); err != nil {
		t.Fatalf("project: %v", err)
	}

	doc, _, err := store.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(doc.Shelves) != 1 {
		t.Fatalf("unresolved book must not create a real-shelf entry; shelves = %v", doc.Shelves)
	}
	if v := doc.Shelves[readingprogress.ReaderShelfID]["book-loose"].Offset; v != 30 {
		t.Fatalf("loose reader entry = %d, want 30", v)
	}
}

// A desktop write does not clobber a newer reader entry the desktop's in-memory
// copy is stale about, in either namespace — recency decides.
func TestWriteReadingProgress_DoesNotClobberNewerReaderEntry(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		// A recent reader entry under the synthetic namespace, and an older
		// desktop-owned real-shelf entry.
		out.Shelves[readingprogress.ReaderShelfID] = map[string]readingprogress.Entry{"book-a": {Offset: 999, At: 2000}}
		out.Shelves["real-shelf"] = map[string]readingprogress.Entry{"book-a": {Offset: 200, At: 1000}}
		return out
	})

	app := &DesktopApp{readingProgressSync: store}
	// A desktop write that advances the real shelf (newer) but carries a stale
	// copy of the reader namespace (older) it read before the reader's last write.
	if err := app.WriteReadingProgress(
		`{"version":2,"shelves":{"real-shelf":{"book-a":{"offset":260,"at":3000}},"book":{"book-a":{"offset":1,"at":500}}}}`,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 260 {
		t.Fatalf("desktop real-shelf offset = %d, want 260 (newer desktop write wins)", v)
	}
	if v := readerOffset(t, store, readingprogress.ReaderShelfID, "book-a"); v != 999 {
		t.Fatalf("reader namespace offset = %d, want 999 (stale desktop copy must not clobber it)", v)
	}
}

// A shelf whose name slugifies to the reader's namespace must not be handed that
// id, or its progress would share (and break) the reader's "book" namespace.
func TestGenerateDesktopShelfIDReservesReaderNamespace(t *testing.T) {
	if id := generateDesktopShelfID("book", map[string]bool{}); id == readingprogress.ReaderShelfID {
		t.Fatalf("shelf id %q collides with the reader namespace", id)
	}
	if id := generateDesktopShelfID("Book", map[string]bool{}); id == readingprogress.ReaderShelfID {
		t.Fatalf("shelf id %q collides with the reader namespace", id)
	}
	// An unrelated name is unaffected.
	if id := generateDesktopShelfID("Comics", map[string]bool{}); id != "comics" {
		t.Fatalf("unexpected shelf id for Comics: %q", id)
	}
}

// The first close request is held (prevent) and the frontend is notified to
// flush; the app does not quit yet.
func TestQuitGate_FirstRequestPreventsAndNotifies(t *testing.T) {
	var emits int32
	quit := make(chan struct{}, 1)
	gate := newQuitGate(time.Second, func() { atomic.AddInt32(&emits, 1) }, func() { quit <- struct{}{} })

	if prevent := gate.requestClose(); !prevent {
		t.Fatal("first close request must be prevented")
	}
	if got := atomic.LoadInt32(&emits); got != 1 {
		t.Fatalf("emit called %d times, want 1", got)
	}
	select {
	case <-quit:
		t.Fatal("must not quit before ack or timeout")
	case <-time.After(50 * time.Millisecond):
	}
}

// Once the frontend acks the flush, the gate quits.
func TestQuitGate_QuitsAfterAck(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := newQuitGate(time.Second, func() {}, func() { quit <- struct{}{} })

	gate.requestClose()
	gate.flushed()

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after ack")
	}
}

// With no ack, the gate quits once the timeout elapses, so an unresponsive
// frontend (or a .lock held by another process) cannot keep the window open.
func TestQuitGate_QuitsAfterTimeout(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := newQuitGate(20*time.Millisecond, func() {}, func() { quit <- struct{}{} })

	gate.requestClose()

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after timeout")
	}
}

// A second close request (the runtime.Quit that follows the first) is let
// through immediately and does not notify the frontend again, so close cannot
// loop on itself.
func TestQuitGate_SecondRequestAllows(t *testing.T) {
	var emits int32
	gate := newQuitGate(time.Second, func() { atomic.AddInt32(&emits, 1) }, func() {})

	if prevent := gate.requestClose(); !prevent {
		t.Fatal("first close request must be prevented")
	}
	if prevent := gate.requestClose(); prevent {
		t.Fatal("second close request must be allowed")
	}
	if got := atomic.LoadInt32(&emits); got != 1 {
		t.Fatalf("emit called %d times across two requests, want 1", got)
	}
}

// A stray ack — before any close request, or a second one — is a harmless
// no-op and must not panic (closing an already-closed channel would).
func TestQuitGate_StrayAckIsNoop(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := newQuitGate(time.Second, func() {}, func() { quit <- struct{}{} })

	gate.flushed() // before any request: nothing to release
	gate.requestClose()
	gate.flushed()
	gate.flushed() // second ack: already released

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after ack")
	}
}

// A corrupt write is rejected rather than lenient-parsed into an empty document
// (which would wipe stored progress).
func TestWriteReadingProgress_RejectsCorruptJSON(t *testing.T) {
	store := readingprogress.NewStore(filepath.Join(t.TempDir(), "reading_progress.json"))
	seedStore(t, store, func(doc readingprogress.Document) readingprogress.Document {
		out := doc.Clone()
		out.Shelves["real-shelf"] = map[string]readingprogress.Entry{"book-a": {Offset: 50, At: 1000}}
		return out
	})

	app := &DesktopApp{readingProgressSync: store}
	if err := app.WriteReadingProgress("{not json"); err == nil {
		t.Fatalf("expected corrupt JSON to be rejected")
	}
	if v := readerOffset(t, store, "real-shelf", "book-a"); v != 50 {
		t.Fatalf("stored progress must survive a rejected write, got %d", v)
	}
}
