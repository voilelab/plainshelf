import { afterEach, describe, expect, it, vi } from 'vitest';
import { bindWails, unbindWails } from '#testing/wailsRuntime';
import {
  hasDesktopReadingProgressBinding,
  openDesktopBookFiles,
  readDesktopReadingProgress,
  stageDesktopReadingProgress,
  writeDesktopReadingProgress
} from './desktop';

vi.mock('@wailsio/runtime', () => import('#testing/wailsRuntime'));

afterEach(() => {
  vi.unstubAllGlobals();
  unbindWails();
});

type Host = '__PLAINSHELF_DESKTOP__' | '__PLAINSHELF_READER__';

function stubMain(main: Parameters<typeof bindWails>[0], host: Host = '__PLAINSHELF_DESKTOP__'): void {
  bindWails(main);
  vi.stubGlobal('window', { [host]: {} });
}

describe('reading-progress binding resolution', () => {
  it('reports no binding outside both app windows', () => {
    vi.stubGlobal('window', {});
    expect(hasDesktopReadingProgressBinding()).toBe(false);
  });

  it('uses the binding in the desktop window', () => {
    stubMain({});
    expect(hasDesktopReadingProgressBinding()).toBe(true);
  });

  it('uses the binding in the reader window', () => {
    stubMain({}, '__PLAINSHELF_READER__');
    expect(hasDesktopReadingProgressBinding()).toBe(true);
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
  it('forwards the position to the reader binding', async () => {
    const stage = vi.fn(async () => undefined);
    stubMain({ ReaderApp: { StageReadingProgress: stage } }, '__PLAINSHELF_READER__');

    stageDesktopReadingProgress('book-a', 120, 2000);

    await vi.waitFor(() => expect(stage).toHaveBeenCalledOnce());
    expect(stage).toHaveBeenCalledWith(expect.any(String), 'book-a', 120, 2000);
  });

  it('swallows a missing binding', async () => {
    stubMain({ ReaderApp: {} }, '__PLAINSHELF_READER__');

    expect(() => stageDesktopReadingProgress('book-a', 120, 2000)).not.toThrow();
    await vi.dynamicImportSettled();
  });
});

describe('desktop calls', () => {
  it('rejects with the method name when the desktop window lacks the binding', async () => {
    stubMain({ DesktopApp: {} });

    await expect(openDesktopBookFiles()).rejects.toThrow('DesktopApp.OpenBookFiles');
  });

  it('returns null outside the desktop window', async () => {
    const openBookFiles = vi.fn(async () => ['a']);
    bindWails({ DesktopApp: { OpenBookFiles: openBookFiles } });
    vi.stubGlobal('window', {});

    expect(await openDesktopBookFiles()).toBeNull();
    expect(openBookFiles).not.toHaveBeenCalled();
  });
});
