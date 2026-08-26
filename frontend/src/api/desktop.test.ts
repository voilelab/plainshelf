import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  hasDesktopReadingProgressBinding,
  readDesktopReadingProgress,
  stageDesktopReadingProgress,
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
});

describe('stageDesktopReadingProgress', () => {
  it('forwards the position to the reader binding', () => {
    const stage = vi.fn(async () => undefined);
    stubMain({
      ReaderApp: {
        ReadReadingProgress: async () => '',
        WriteReadingProgress: async () => undefined,
        StageReadingProgress: stage
      }
    });

    stageDesktopReadingProgress('book-a', 120, 2000);

    expect(stage).toHaveBeenCalledOnce();
    // shelfID (from the active client shelf), bookID, offset, at.
    expect(stage).toHaveBeenCalledWith(expect.any(String), 'book-a', 120, 2000);
  });
});
