import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  hasDesktopReadingProgressBinding,
  openDesktopBookFiles,
  readDesktopReadingProgress,
  stageDesktopReadingProgress,
  writeDesktopReadingProgress
} from './desktop';

afterEach(() => {
  vi.unstubAllGlobals();
});

type Host = '__PLAINSHELF_DESKTOP__' | '__PLAINSHELF_READER__';

function stubMain(main: Record<string, unknown>, host: Host = '__PLAINSHELF_DESKTOP__'): void {
  vi.stubGlobal('window', { go: { main }, [host]: {} });
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

  it('detects the standalone reader binding in the reader window', () => {
    stubMain(
      {
        ReaderApp: {
          ReadReadingProgress: async () => '',
          WriteReadingProgress: async () => undefined
        }
      },
      '__PLAINSHELF_READER__'
    );
    expect(hasDesktopReadingProgressBinding()).toBe(true);
  });

  it('ignores the reader binding inside the desktop window', () => {
    stubMain({
      ReaderApp: {
        ReadReadingProgress: async () => '',
        WriteReadingProgress: async () => undefined
      }
    });
    expect(hasDesktopReadingProgressBinding()).toBe(false);
  });

  it('routes read and write to the reader binding in the reader window', async () => {
    const read = vi.fn(async () => '{"version":1,"shelves":{"book":{"b":5}}}');
    const write = vi.fn(async () => undefined);
    stubMain(
      { ReaderApp: { ReadReadingProgress: read, WriteReadingProgress: write } },
      '__PLAINSHELF_READER__'
    );

    expect(await readDesktopReadingProgress()).toContain('"book"');
    expect(read).toHaveBeenCalledOnce();

    await writeDesktopReadingProgress('{"version":1,"shelves":{}}');
    expect(write).toHaveBeenCalledOnce();
  });

  it('routes to the desktop app binding in the desktop window', async () => {
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
    stubMain(
      {
        ReaderApp: {
          ReadReadingProgress: async () => '',
          WriteReadingProgress: async () => undefined,
          StageReadingProgress: stage
        }
      },
      '__PLAINSHELF_READER__'
    );

    stageDesktopReadingProgress('book-a', 120, 2000);

    expect(stage).toHaveBeenCalledOnce();
    expect(stage).toHaveBeenCalledWith(expect.any(String), 'book-a', 120, 2000);
  });
});

describe('desktop calls', () => {
  it('rejects with the method name when the desktop window lacks the binding', async () => {
    stubMain({ DesktopApp: {} });

    await expect(openDesktopBookFiles()).rejects.toThrow('DesktopApp.OpenBookFiles');
  });

  it('returns null outside the desktop window', async () => {
    vi.stubGlobal('window', { go: { main: { DesktopApp: { OpenBookFiles: async () => ['a'] } } } });

    expect(await openDesktopBookFiles()).toBeNull();
  });
});
