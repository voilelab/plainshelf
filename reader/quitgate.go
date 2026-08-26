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
// It has to cover a real save: the shared reading-progress store is a
// read-modify-write, so one save is two IPC round trips plus a flock that can
// queue behind the desktop app — 1.5s leaves room for that without letting a
// wedged frontend hold the window open indefinitely.
const quitGateTimeout = 1500 * time.Millisecond

// quitGate coordinates a graceful window close. Every close request is held
// back while the frontend flushes the reading position; the window is released
// only once the frontend acks or a timeout elapses — whichever comes first — so
// the app closes under every condition, including an unresponsive frontend or a
// .lock held by another process.
//
// Two flags, not one: quitting marks that a flush is under way, and releasing
// marks that the gate has committed to going down. A second *user* close during
// the flush (a double-click, or Cmd+Q while the window is still visible) must
// keep being held — letting it straight through would close the window with the
// save still in flight, the very loss the gate exists to prevent. Only the
// gate's own runtime.Quit, which re-enters OnBeforeClose after releasing is set,
// is passed through, so the gate releases itself but nothing else does.
//
// It is Wails-agnostic: emit notifies the frontend (runtime.EventsEmit) and
// quit tears the window down (runtime.Quit). quit runs on its own goroutine so
// the OnBeforeClose callback returns promptly.
//
// This is a deliberate copy of desktop/quitgate.go: the reader is an
// independent Wails app in its own module, so the two cannot share a package.
type quitGate struct {
	timeout time.Duration
	emit    func()
	quit    func()

	mu        sync.Mutex
	quitting  bool
	releasing bool
	ack       chan struct{}
}

func newQuitGate(timeout time.Duration, emit, quit func()) *quitGate {
	return &quitGate{timeout: timeout, emit: emit, quit: quit}
}

// requestClose is the OnBeforeClose body. It returns false only once the gate is
// releasing — its own runtime.Quit re-entering OnBeforeClose — and true for
// every other request: the first starts the flush, and any close while the
// flush is still running is held so the save is not cut short.
func (g *quitGate) requestClose() (prevent bool) {
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
