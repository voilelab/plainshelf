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
    vi.stubGlobal('window', {
      __PLAINSHELF_DESKTOP__: true,
      go: { main: { DesktopApp: { ListShelfDetails, RetryShelf } } }
    });
    const native = createWailsPlatform();
    await expect(native.shelves?.listShelfDetails?.()).resolves.toEqual([]);
    await native.shelves?.retryShelf?.('remote');
    expect(ListShelfDetails).toHaveBeenCalledOnce();
    expect(RetryShelf).toHaveBeenCalledWith('remote');
  } finally {
    vi.unstubAllGlobals();
  }
});
