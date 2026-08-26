package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// The reader's quit gate is a copy of the desktop's; these mirror
// desktop/reading_progress_test.go so a drift in the copy is caught here.

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

// With no ack, the gate quits once the timeout elapses.
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

// A second close request is let through immediately and does not notify the
// frontend again, so close cannot loop on itself.
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
// no-op and must not panic.
func TestQuitGate_StrayAckIsNoop(t *testing.T) {
	quit := make(chan struct{}, 1)
	gate := newQuitGate(time.Second, func() {}, func() { quit <- struct{}{} })

	gate.flushed()
	gate.requestClose()
	gate.flushed()
	gate.flushed()

	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("gate did not quit after ack")
	}
}
