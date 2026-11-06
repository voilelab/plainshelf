import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { bindWails, emitWailsEvent, unbindWails } from '#testing/wailsRuntime';

const { openDesktopReaderMock } = vi.hoisted(() => ({
  openDesktopReaderMock: vi.fn()
}));

vi.mock('@/api/desktop', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/desktop')>();
  return {
    ...actual,
    openDesktopReader: openDesktopReaderMock
  };
});

vi.mock('@wailsio/runtime', () => import('#testing/wailsRuntime'));

import { createWailsPlatform } from './wailsBookshelfProvider';

describe('WailsPlatform.openReader', () => {
  beforeEach(() => {
    openDesktopReaderMock.mockReset();
  });

  it('passes the book and section to the launch', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);

    await createWailsPlatform().openReader!('book-1', 3);

    expect(openDesktopReaderMock).toHaveBeenCalledWith('book-1', 3);
  });

  it('rethrows the launch error unchanged', async () => {
    const launchError = new Error('reader_unsupported_platform');
    openDesktopReaderMock.mockRejectedValue(launchError);

    await expect(createWailsPlatform().openReader!('book-1')).rejects.toBe(launchError);
  });
});

it('exposes virtual-shelf management only when the native bindings exist', async () => {
  try {
    vi.stubGlobal('window', { location: { search: '?desktop-shell-preview=1' } });
    const preview = createWailsPlatform();
    expect(preview.shelves?.listShelfDetails).toBeUndefined();
    expect(preview.shelves?.retryShelf).toBeUndefined();

    const ListShelfDetails = vi.fn().mockResolvedValue([]);
    const RetryShelf = vi.fn().mockResolvedValue(undefined);
    bindWails({ DesktopApp: { ListShelfDetails, RetryShelf } });
    vi.stubGlobal('window', { __PLAINSHELF_DESKTOP__: true });
    const native = createWailsPlatform();
    await expect(native.shelves?.listShelfDetails?.()).resolves.toEqual([]);
    await native.shelves?.retryShelf?.('remote');
    expect(ListShelfDetails).toHaveBeenCalledOnce();
    expect(RetryShelf).toHaveBeenCalledWith('remote');
  } finally {
    vi.unstubAllGlobals();
    unbindWails();
  }
});

describe('desktop reader windows', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    unbindWails();
  });

  it('are offered only inside the desktop app', () => {
    vi.stubGlobal('window', { location: { search: '?desktop-shell-preview=1' } });
    const preview = createWailsPlatform();

    expect(preview.openReaderWindow).toBeUndefined();
    expect(preview.onReadingDataChanged).toBeUndefined();
  });

  it('open the book and section in a new window', async () => {
    const OpenReaderWindow = vi.fn().mockResolvedValue(undefined);
    bindWails({ DesktopApp: { OpenReaderWindow } });
    vi.stubGlobal('window', { __PLAINSHELF_DESKTOP__: true });

    await createWailsPlatform().openReaderWindow!('book-1', 3);

    expect(OpenReaderWindow).toHaveBeenCalledWith(expect.any(String), 'book-1', 3);
  });

  it('relay reading-data changes until unsubscribed', async () => {
    vi.stubGlobal('window', { __PLAINSHELF_DESKTOP__: true });
    const listener = vi.fn();

    const stop = createWailsPlatform().onReadingDataChanged!(listener);
    await vi.dynamicImportSettled();
    emitWailsEvent('plainshelf:reading-data-changed');
    stop();
    emitWailsEvent('plainshelf:reading-data-changed');

    expect(listener).toHaveBeenCalledOnce();
  });
});
