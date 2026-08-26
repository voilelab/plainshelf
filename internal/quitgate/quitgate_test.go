package quitgate

import (
	"sync/atomic"
	"testing"
	"time"
)

// The first close request is held (prevent) and the frontend is notified to
// flush; the app does not quit yet.
func TestGate_FirstRequestPreventsAndNotifies(t *testing.T) {
	var emits int32
	quit := make(chan struct{}, 1)
	gate := New(time.Second, func() { atomic.AddInt32(&emits, 1) }, func() { quit <- struct{}{} })

	if prevent := gate.RequestClose(); !prevent {
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
func TestGate_QuitsAfterAck(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := New(time.Second, func() {}, func() { quit <- struct{}{} })

	gate.RequestClose()
	gate.Flushed()

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after ack")
	}
}

// With no ack, the gate quits once the timeout elapses, so an unresponsive
// frontend (or a .lock held by another process) cannot keep the window open.
func TestGate_QuitsAfterTimeout(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := New(20*time.Millisecond, func() {}, func() { quit <- struct{}{} })

	gate.RequestClose()

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after timeout")
	}
}

// A second user close during the flush is still held — closing then would cut
// the save short — and does not notify the frontend again.
func TestGate_SubsequentUserCloseStaysHeld(t *testing.T) {
	var emits int32
	quit := make(chan struct{}, 1)
	// Long timeout so the gate does not release on its own during the test.
	gate := New(time.Second, func() { atomic.AddInt32(&emits, 1) }, func() { quit <- struct{}{} })

	if prevent := gate.RequestClose(); !prevent {
		t.Fatal("first close request must be prevented")
	}
	if prevent := gate.RequestClose(); !prevent {
		t.Fatal("a close during the flush must still be prevented")
	}
	if got := atomic.LoadInt32(&emits); got != 1 {
		t.Fatalf("emit called %d times, want 1 (no re-notify while flushing)", got)
	}
	select {
	case <-quit:
		t.Fatal("must not quit while the flush is still held")
	case <-time.After(50 * time.Millisecond):
	}
}

// Only the gate's own runtime.Quit — which re-enters RequestClose after the ack
// — is let through, so the app closes exactly once without looping on itself.
func TestGate_OwnQuitReentryIsAllowed(t *testing.T) {
	var gate *Gate
	done := make(chan struct{})
	var reentryPrevent bool
	gate = New(time.Second, func() {}, func() {
		// Emulate runtime.Quit re-triggering OnBeforeClose.
		reentryPrevent = gate.RequestClose()
		close(done)
	})

	gate.RequestClose()
	gate.Flushed()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after ack")
	}
	if reentryPrevent {
		t.Fatal("the gate's own quit re-entry must be allowed (prevent=false)")
	}
}

// A stray ack — before any close request, or a second one — is a harmless
// no-op and must not panic (closing an already-closed channel would).
func TestGate_StrayAckIsNoop(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := New(time.Second, func() {}, func() { quit <- struct{}{} })

	gate.Flushed() // before any request: nothing to release
	gate.RequestClose()
	gate.Flushed()
	gate.Flushed() // second ack: already released

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after ack")
	}
}
