import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The module keeps process-wide state (the single listener and the handler set),
// so each test loads a fresh copy after stubbing window.
async function loadModule() {
  vi.resetModules();
  return import('./desktopQuitGate');
}

interface WindowStub {
  runtime?: { EventsOn: ReturnType<typeof vi.fn> };
  go?: { main?: { DesktopApp?: unknown; ReaderApp?: unknown } };
  // ./desktop pulls in ./client, which reads localStorage at import time.
  localStorage: { getItem: () => string | null };
}

function makeWindow(withRuntime: boolean): {
  windowStub: WindowStub;
  ack: ReturnType<typeof vi.fn>;
  fireBeforeClose: () => void;
} {
  const ack = vi.fn().mockResolvedValue(undefined);
  let captured: (() => void) | null = null;
  const windowStub: WindowStub = {
    go: { main: { ReaderApp: { AckBeforeClose: ack } } },
    localStorage: { getItem: () => null }
  };
  if (withRuntime) {
    windowStub.runtime = {
      EventsOn: vi.fn((name: string, cb: () => void) => {
        if (name === 'app:before-close') {
          captured = cb;
        }
      })
    };
  }
  return {
    windowStub,
    ack,
    fireBeforeClose: () => captured?.()
  };
}

describe('desktopQuitGate', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('does nothing outside a Wails runtime (no window.runtime)', async () => {
    const { windowStub } = makeWindow(false);
    vi.stubGlobal('window', windowStub);
    const { installDesktopQuitGate } = await loadModule();

    expect(() => installDesktopQuitGate()).not.toThrow();
  });

  it('acks immediately when no flush is registered', async () => {
    const { windowStub, ack, fireBeforeClose } = makeWindow(true);
    vi.stubGlobal('window', windowStub);
    const { installDesktopQuitGate } = await loadModule();

    installDesktopQuitGate();
    fireBeforeClose();
    await vi.waitFor(() => expect(ack).toHaveBeenCalledTimes(1));
  });

  it('runs the registered flush before acking', async () => {
    const { windowStub, ack, fireBeforeClose } = makeWindow(true);
    vi.stubGlobal('window', windowStub);
    const { installDesktopQuitGate, registerBeforeCloseFlush } = await loadModule();

    const order: string[] = [];
    const flush = vi.fn(async () => {
      order.push('flush');
    });
    ack.mockImplementation(async () => {
      order.push('ack');
    });

    installDesktopQuitGate();
    registerBeforeCloseFlush(flush);
    fireBeforeClose();

    await vi.waitFor(() => expect(ack).toHaveBeenCalledTimes(1));
    expect(flush).toHaveBeenCalledTimes(1);
    expect(order).toEqual(['flush', 'ack']);
  });

  it('acks even when a flush rejects, so the window still closes', async () => {
    const { windowStub, ack, fireBeforeClose } = makeWindow(true);
    vi.stubGlobal('window', windowStub);
    const { installDesktopQuitGate, registerBeforeCloseFlush } = await loadModule();

    registerBeforeCloseFlush(() => Promise.reject(new Error('flush failed')));
    installDesktopQuitGate();
    fireBeforeClose();

    await vi.waitFor(() => expect(ack).toHaveBeenCalledTimes(1));
  });

  it('does not run a flush after it is unregistered', async () => {
    const { windowStub, ack, fireBeforeClose } = makeWindow(true);
    vi.stubGlobal('window', windowStub);
    const { installDesktopQuitGate, registerBeforeCloseFlush } = await loadModule();

    const flush = vi.fn().mockResolvedValue(undefined);
    const unregister = registerBeforeCloseFlush(flush);
    installDesktopQuitGate();
    unregister();
    fireBeforeClose();

    await vi.waitFor(() => expect(ack).toHaveBeenCalledTimes(1));
    expect(flush).not.toHaveBeenCalled();
  });

  it('registers the listener only once across repeated installs', async () => {
    const { windowStub } = makeWindow(true);
    vi.stubGlobal('window', windowStub);
    const { installDesktopQuitGate } = await loadModule();

    installDesktopQuitGate();
    installDesktopQuitGate();

    expect(windowStub.runtime?.EventsOn).toHaveBeenCalledTimes(1);
  });
});
