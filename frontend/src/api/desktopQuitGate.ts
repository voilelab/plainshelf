import { ackDesktopBeforeClose } from './desktop';

// The frontend side of the native quit gate (desktop/quitgate.go,
// reader/quitgate.go). On the desktop and reader apps the native shell holds
// the first window close and emits `app:before-close`; the app has that long —
// bounded by the gate's timeout — to flush the reading position and ack, so the
// last few seconds of scrolling are not lost when the window is closed.
//
// A single listener serves the whole app. Screens that hold unsaved state (the
// reader) register a flush through registerBeforeCloseFlush; when none is
// registered — the library page, say — the handler acks immediately, so closing
// away from the reader is not delayed.

const BEFORE_CLOSE_EVENT = 'app:before-close';

type FlushHandler = () => Promise<void>;

// Wails injects its runtime as window.runtime. Only the one method used here is
// typed; it is absent in a plain browser (and in the desktop-shell preview),
// where installDesktopQuitGate simply does nothing.
interface WailsRuntimeWindow extends Window {
  runtime?: {
    EventsOn?: (eventName: string, callback: (...data: unknown[]) => void) => void;
  };
}

const flushHandlers = new Set<FlushHandler>();
let installed = false;

/**
 * Registers a flush to run before the app closes. Returns an unregister
 * function; call it when the owning screen tears down so a stale flush does not
 * outlive it. Safe to call in any runtime — the flush only ever runs on the
 * desktop and reader apps, where the native close event fires.
 */
export function registerBeforeCloseFlush(handler: FlushHandler): () => void {
  flushHandlers.add(handler);
  return () => {
    flushHandlers.delete(handler);
  };
}

async function runRegisteredFlushes(): Promise<void> {
  // allSettled, not all: one screen's failed flush must not stop the others
  // from running, and the app has to close regardless of any flush outcome.
  await Promise.allSettled([...flushHandlers].map((handler) => handler()));
}

/**
 * Installs the single `app:before-close` listener. Idempotent, and a no-op
 * outside a Wails runtime (no window.runtime), so it is safe to call
 * unconditionally at startup.
 *
 * The listener always acks, even when a flush throws, so the native gate is
 * released and the window closes rather than waiting out its full timeout.
 */
export function installDesktopQuitGate(): void {
  if (installed) {
    return;
  }
  const runtime = (window as WailsRuntimeWindow).runtime;
  if (!runtime?.EventsOn) {
    return;
  }
  installed = true;

  runtime.EventsOn(BEFORE_CLOSE_EVENT, () => {
    void (async () => {
      try {
        await runRegisteredFlushes();
      } finally {
        await ackDesktopBeforeClose();
      }
    })();
  });
}
