// Package quitgate coordinates a graceful window close for the Wails desktop
// app and the standalone reader. Both hold their window open while the frontend
// flushes the reading position and only then quit, so the last stretch of
// reading since the previous autosave is not lost when the window is closed.
//
// The two apps are independent Wails modules with their own main packages; this
// is the one piece of that dance they share. It is Wails-agnostic — the caller
// injects what notifies the frontend and what tears the window down — so it
// carries no UI dependency and is tested on its own.
package quitgate

import (
	"sync"
	"time"
)

// BeforeCloseEvent is emitted to the frontend when a window close is requested,
// asking it to flush the reading position before the app goes down. The
// frontend listens for it (frontend/src/api/desktopQuitGate.ts) and acks
// through the AckBeforeClose binding once its flush completes.
const BeforeCloseEvent = "app:before-close"

// DefaultTimeout bounds how long a close waits for the frontend's flush ack. It
// has to cover a real save: the shared reading-progress store is a
// read-modify-write, so one save is two IPC round trips plus a flock that can
// queue behind the other process — 1.5s leaves room for that without letting a
// wedged frontend hold the window open indefinitely.
const DefaultTimeout = 1500 * time.Millisecond

// Gate coordinates a graceful window close. Every close request is held back
// while the frontend flushes the reading position; the window is released only
// once the frontend acks or a timeout elapses — whichever comes first — so the
// app closes under every condition, including an unresponsive frontend or a
// .lock held by another process.
//
// Two flags, not one: quitting marks that a flush is under way, and releasing
// marks that the gate has committed to going down. A second *user* close during
// the flush (a double-click, or Cmd+Q while the window is still visible) must
// keep being held — letting it straight through would close the window with the
// save still in flight, the very loss the gate exists to prevent. Only the
// gate's own runtime.Quit, which re-enters RequestClose after releasing is set,
// is passed through, so the gate releases itself but nothing else does.
//
// emit notifies the frontend (runtime.EventsEmit) and quit tears the window
// down (runtime.Quit). quit runs on its own goroutine so the OnBeforeClose
// callback returns promptly.
type Gate struct {
	timeout time.Duration
	emit    func()
	quit    func()

	mu        sync.Mutex
	quitting  bool
	releasing bool
	ack       chan struct{}
}

// New builds a Gate. timeout bounds the wait for the flush ack (see
// DefaultTimeout); emit is called to ask the frontend to flush, and quit to
// close the window once the flush is done or the timeout elapses.
func New(timeout time.Duration, emit, quit func()) *Gate {
	return &Gate{timeout: timeout, emit: emit, quit: quit}
}

// RequestClose is the OnBeforeClose body. It returns false only once the gate is
// releasing — its own runtime.Quit re-entering OnBeforeClose — and true for
// every other request: the first starts the flush, and any close while the
// flush is still running is held so the save is not cut short.
func (g *Gate) RequestClose() (prevent bool) {
	g.mu.Lock()
	if g.releasing {
		g.mu.Unlock()
		return false
	}
	if g.quitting {
		g.mu.Unlock()
		return true
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
		g.mu.Lock()
		g.releasing = true
		g.mu.Unlock()
		g.quit()
	}()

	return true
}

// Flushed is the ack path the frontend drives once it has finished flushing. It
// releases the held close. A call before any close request, or a second call,
// is a no-op — the window closes exactly once.
func (g *Gate) Flushed() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.quitting || g.ack == nil {
		return
	}
	close(g.ack)
	g.ack = nil
}
