import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  hasDesktopReadingProgressBinding,
  readDesktopReadingProgress,
  writeDesktopReadingProgress
} from './desktop';

afterEach(() => {
  vi.unstubAllGlobals();
});

function stubMain(main: Record<string, unknown>): void {
  vi.stubGlobal('window', { go: { main } });
}

describe('reading-progress binding resolution', () => {
  it('reports no binding when neither app is present', () => {
    stubMain({});
    expect(hasDesktopReadingProgressBinding()).toBe(false);
  });

  it('detects the desktop app binding', () => {
    stubMain({
      DesktopApp: {
        ReadReadingProgress: async () => '',
        WriteReadingProgress: async () => undefined
      }
    });
    expect(hasDesktopReadingProgressBinding()).toBe(true);
  });

  it('detects the standalone reader binding (window.go.main.ReaderApp)', () => {
    stubMain({
      ReaderApp: {
        ReadReadingProgress: async () => '',
        WriteReadingProgress: async () => undefined
      }
    });
    // Without this, the reader falls back to localStorage and its progress never
    // reaches the shared reading_progress.json.
    expect(hasDesktopReadingProgressBinding()).toBe(true);
  });

  it('routes read and write to the reader binding when only it is present', async () => {
    const read = vi.fn(async () => '{"version":1,"shelves":{"book":{"b":5}}}');
    const write = vi.fn(async () => undefined);
    stubMain({ ReaderApp: { ReadReadingProgress: read, WriteReadingProgress: write } });

    expect(await readDesktopReadingProgress()).toContain('"book"');
    expect(read).toHaveBeenCalledOnce();

    await writeDesktopReadingProgress('{"version":1,"shelves":{}}');
    expect(write).toHaveBeenCalledOnce();
  });

  it('prefers the desktop app binding when both are present', async () => {
    const desktopWrite = vi.fn(async () => undefined);
    const readerWrite = vi.fn(async () => undefined);
    stubMain({
      DesktopApp: { ReadReadingProgress: async () => '', WriteReadingProgress: desktopWrite },
      ReaderApp: { ReadReadingProgress: async () => '', WriteReadingProgress: readerWrite }
    });

    await writeDesktopReadingProgress('{"version":1,"shelves":{}}');
    expect(desktopWrite).toHaveBeenCalledOnce();
    expect(readerWrite).not.toHaveBeenCalled();
  });

  // The whole point of routing the reader to the shared file before its binding is
  // bound: read/write must wait for a late-injected binding instead of throwing,
  // which the reader's fetchReaderData would turn into a failed book load.
  describe('late-injected binding', () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it('reads once the binding is injected rather than throwing', async () => {
      vi.useFakeTimers();
      // Reader window is up, but window.go is not there yet.
      vi.stubGlobal('window', {});

      const read = vi.fn(async () => '{"version":2,"shelves":{"book":{"b":5}}}');
      const pending = readDesktopReadingProgress();
      const settled = vi.fn();
      void pending.then(settled);

      // Still nothing bound: the call is waiting, not rejected.
      await vi.advanceTimersByTimeAsync(120);
      expect(settled).not.toHaveBeenCalled();
      expect(read).not.toHaveBeenCalled();

      // Wails injects the reader binding; the next poll picks it up.
      stubMain({ ReaderApp: { ReadReadingProgress: read, WriteReadingProgress: async () => undefined } });
      await vi.advanceTimersByTimeAsync(60);

      await expect(pending).resolves.toContain('"book"');
      expect(read).toHaveBeenCalledOnce();
    });

    it('writes once the binding is injected rather than dropping the write', async () => {
      vi.useFakeTimers();
      vi.stubGlobal('window', {});

      const write = vi.fn(async () => undefined);
      const pending = writeDesktopReadingProgress('{"version":2,"shelves":{}}');

      await vi.advanceTimersByTimeAsync(120);
      expect(write).not.toHaveBeenCalled();

      stubMain({ ReaderApp: { ReadReadingProgress: async () => '', WriteReadingProgress: write } });
      await vi.advanceTimersByTimeAsync(60);

      await expect(pending).resolves.toBeUndefined();
      expect(write).toHaveBeenCalledOnce();
    });
  });
});
