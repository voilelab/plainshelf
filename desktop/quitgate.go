package main

import (
	"sync"
	"time"
)

// beforeCloseEvent is emitted to the frontend when a window close is requested,
// asking it to flush the reading position it is holding before the app goes
// down. The frontend listens for it (frontend/src/api/desktopQuitGate.ts) and
// acks through the AckBeforeClose binding once its flush completes.
const beforeCloseEvent = "app:before-close"

// quitGateTimeout bounds how long a close waits for the frontend's flush ack.
// It has to cover a real save: DeviceDocumentStore.mutate() is a
// read-modify-write, so one save is two IPC round trips plus a flock that can
// queue behind another process — 1.5s leaves room for that without letting a
// wedged frontend hold the window open indefinitely.
const quitGateTimeout = 1500 * time.Millisecond

// quitGate coordinates a graceful window close. The first close request is held
// back while the frontend flushes the reading position; the window is released
// once the frontend acks or a timeout elapses — whichever comes first — so the
// app closes under every condition, including an unresponsive frontend or a
// .lock held by another process.
//
// It is Wails-agnostic: emit notifies the frontend (runtime.EventsEmit) and
// quit tears the window down (runtime.Quit). quit runs on its own goroutine so
// the OnBeforeClose callback returns promptly, and quit re-enters requestClose
// (runtime.Quit triggers OnBeforeClose again) — the quitting flag lets that
// second pass through so the gate never holds itself closed.
type quitGate struct {
	timeout time.Duration
	emit    func()
	quit    func()

	mu       sync.Mutex
	quitting bool
	ack      chan struct{}
}

func newQuitGate(timeout time.Duration, emit, quit func()) *quitGate {
	return &quitGate{timeout: timeout, emit: emit, quit: quit}
}

// requestClose is the OnBeforeClose body. It returns true to hold the first
// close request (while the frontend flushes) and false on every later request,
// so the eventual runtime.Quit — which re-enters OnBeforeClose — is let through
// and the app cannot deadlock holding itself closed.
func (g *quitGate) requestClose() (prevent bool) {
	g.mu.Lock()
	if g.quitting {
		g.mu.Unlock()
		return false
	}
	g.quitting = true
	g.ack = make(chan struct{})
	ack := g.ack
	g.mu.Unlock()

	g.emit()

	go func() {
		timer := time.NewTimer(g.timeout)
		defer timer.Stop()
		select {
		case <-ack:
		case <-timer.C:
		}
		g.quit()
	}()

	return true
}

// flushed is the ack path the frontend drives once it has finished flushing. It
// releases the held close. A call before any close request, or a second call,
// is a no-op — the window closes exactly once.
func (g *quitGate) flushed() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.quitting || g.ack == nil {
		return
	}
	close(g.ack)
	g.ack = nil
}
