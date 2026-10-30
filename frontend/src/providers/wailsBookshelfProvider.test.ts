import { beforeEach, describe, expect, it, vi } from 'vitest';

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

import { createWailsPlatform, WailsBookshelfProvider } from './wailsBookshelfProvider';

describe('WailsBookshelfProvider — read history on shell-out', () => {
  beforeEach(() => {
    openDesktopReaderMock.mockReset();
  });

  function makeProvider(): { provider: WailsBookshelfProvider; addReadHistory: ReturnType<typeof vi.fn> } {
    const provider = new WailsBookshelfProvider();
    const addReadHistory = vi.fn().mockResolvedValue(undefined);
    provider.addReadHistory = addReadHistory;
    return { provider, addReadHistory };
  }

  it('records read history after a successful launch', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);
    const { provider, addReadHistory } = makeProvider();

    await createWailsPlatform(provider).openReader!('book-1', 3);

    expect(openDesktopReaderMock).toHaveBeenCalledWith('book-1', 3);
    expect(addReadHistory).toHaveBeenCalledWith('book-1');
  });

  it('does not record and rethrows unchanged when the launch rejects', async () => {
    const launchError = new Error('reader_unsupported_platform');
    openDesktopReaderMock.mockRejectedValue(launchError);
    const { provider, addReadHistory } = makeProvider();

    await expect(createWailsPlatform(provider).openReader!('book-1')).rejects.toBe(launchError);
    expect(addReadHistory).not.toHaveBeenCalled();
  });

  it('still resolves when the read-history write rejects', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);
    const { provider, addReadHistory } = makeProvider();
    addReadHistory.mockRejectedValue(new Error('storage unavailable'));
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    await expect(createWailsPlatform(provider).openReader!('book-1')).resolves.toBeUndefined();
    expect(addReadHistory).toHaveBeenCalledWith('book-1');
    expect(warnSpy).toHaveBeenCalled();

    warnSpy.mockRestore();
  });

  it('still resolves when the read-history write throws synchronously', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);
    const { provider, addReadHistory } = makeProvider();
    addReadHistory.mockImplementation(() => {
      throw new Error('No shelf selected.');
    });
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    await expect(createWailsPlatform(provider).openReader!('book-1')).resolves.toBeUndefined();
    expect(warnSpy).toHaveBeenCalled();

    warnSpy.mockRestore();
  });
});

it('exposes virtual-shelf management only when the native bindings exist', async () => {
  try {
    vi.stubGlobal('window', { location: { search: '?desktop-shell-preview=1' } });
    const preview = createWailsPlatform(new WailsBookshelfProvider());
    expect(preview.shelves?.listShelfDetails).toBeUndefined();
    expect(preview.shelves?.retryShelf).toBeUndefined();

    const ListShelfDetails = vi.fn().mockResolvedValue([]);
    const RetryShelf = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('window', { go: { main: { DesktopApp: { ListShelfDetails, RetryShelf } } } });
    const native = createWailsPlatform(new WailsBookshelfProvider());
    await expect(native.shelves?.listShelfDetails?.()).resolves.toEqual([]);
    await native.shelves?.retryShelf?.('remote');
    expect(ListShelfDetails).toHaveBeenCalledOnce();
    expect(RetryShelf).toHaveBeenCalledWith('remote');
  } finally {
    vi.unstubAllGlobals();
  }
});
