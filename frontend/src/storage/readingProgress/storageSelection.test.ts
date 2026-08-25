import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The backend the reading-progress store latches onto is chosen once, at first
// access, by createReadingProgressStorage(). These tests pin that choice against
// Wails binding timing: the reader must pick the shared file even when its
// ReaderApp binding has not been injected yet, or its writes silently divert to
// WebView localStorage and never reach the desktop library.

const isMockApiMode = vi.fn<() => boolean>();
const isReaderRuntime = vi.fn<() => boolean>();
const isWailsRuntime = vi.fn<() => boolean>();
const hasDesktopReadingProgressBinding = vi.fn<() => boolean>();
const writeDesktopReadingProgress = vi.fn<(doc: string) => Promise<void>>();
const readDesktopReadingProgress = vi.fn<() => Promise<string>>();

vi.mock('@/api/client', () => ({
  isMockApiMode: () => isMockApiMode(),
  getApiBase: () => '',
  getActiveShelfID: () => 'shelf-1'
}));

vi.mock('@/providers/runtime', () => ({
  isReaderRuntime: () => isReaderRuntime(),
  isWailsRuntime: () => isWailsRuntime()
}));

vi.mock('@/api/desktop', () => ({
  hasDesktopReadingProgressBinding: () => hasDesktopReadingProgressBinding(),
  writeDesktopReadingProgress: (doc: string) => writeDesktopReadingProgress(doc),
  readDesktopReadingProgress: () => readDesktopReadingProgress()
}));

import { createReadingProgressStorage } from './index';

beforeEach(() => {
  isMockApiMode.mockReturnValue(false);
  isReaderRuntime.mockReturnValue(false);
  isWailsRuntime.mockReturnValue(false);
  hasDesktopReadingProgressBinding.mockReturnValue(false);
  writeDesktopReadingProgress.mockResolvedValue(undefined);
  readDesktopReadingProgress.mockResolvedValue('');
});

afterEach(() => {
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe('createReadingProgressStorage backend selection', () => {
  it('routes the standalone reader to the shared file even before its Wails binding is bound', async () => {
    // The reader window is up (isReaderRuntime is set from index.html), but the
    // ReaderApp binding has not been injected yet, so the binding probe is false.
    isReaderRuntime.mockReturnValue(true);
    isWailsRuntime.mockReturnValue(true);
    hasDesktopReadingProgressBinding.mockReturnValue(false);

    const storage = createReadingProgressStorage();
    await storage.save('{"version":2,"shelves":{}}');

    // The save must land on the shared-file binding — which by save time is
    // present — rather than being latched onto WebView localStorage.
    expect(writeDesktopReadingProgress).toHaveBeenCalledTimes(1);
  });

  it('routes the desktop client to the shared file once its binding is present', async () => {
    isReaderRuntime.mockReturnValue(false);
    isWailsRuntime.mockReturnValue(true);
    hasDesktopReadingProgressBinding.mockReturnValue(true);

    const storage = createReadingProgressStorage();
    await storage.save('{"version":2,"shelves":{}}');

    expect(writeDesktopReadingProgress).toHaveBeenCalledTimes(1);
  });

  it('keeps a browser desktop-shell preview (no bindings) on localStorage', async () => {
    // isWailsRuntime() is true for ?desktop-shell-preview=1, but there is never a
    // binding: this must not divert to the desktop backend.
    isReaderRuntime.mockReturnValue(false);
    isWailsRuntime.mockReturnValue(true);
    hasDesktopReadingProgressBinding.mockReturnValue(false);

    const setItem = vi.fn();
    vi.stubGlobal('window', {
      localStorage: { getItem: () => null, setItem }
    });

    const storage = createReadingProgressStorage();
    await storage.save('{"version":2,"shelves":{}}');

    expect(writeDesktopReadingProgress).not.toHaveBeenCalled();
    expect(setItem).toHaveBeenCalledTimes(1);
  });

  it('keeps a plain web build on localStorage', async () => {
    isReaderRuntime.mockReturnValue(false);
    isWailsRuntime.mockReturnValue(false);
    hasDesktopReadingProgressBinding.mockReturnValue(false);

    const setItem = vi.fn();
    vi.stubGlobal('window', {
      localStorage: { getItem: () => null, setItem }
    });

    const storage = createReadingProgressStorage();
    await storage.save('{"version":2,"shelves":{}}');

    expect(writeDesktopReadingProgress).not.toHaveBeenCalled();
    expect(setItem).toHaveBeenCalledTimes(1);
  });
});
