import { beforeEach, describe, expect, it, vi } from 'vitest';

// The desktop bindings are the launch side of the shell-out; the provider calls
// openDesktopReader() and, only once it resolves, writes the read-history entry
// the standalone reader cannot write back to this app. Stub the whole module so
// the launch outcome is under the test's control.
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

import { WailsBookshelfProvider } from './wailsBookshelfProvider';

describe('WailsBookshelfProvider — read history on shell-out', () => {
  beforeEach(() => {
    openDesktopReaderMock.mockReset();
  });

  function makeProvider(): { provider: WailsBookshelfProvider; addReadHistory: ReturnType<typeof vi.fn> } {
    const provider = new WailsBookshelfProvider();
    // addReadHistory is inherited from ServerBookshelfProvider and writes to
    // device-local storage; stub it so these tests only assert the wiring in
    // openDesktopReader, not the storage layer.
    const addReadHistory = vi.fn().mockResolvedValue(undefined);
    provider.addReadHistory = addReadHistory;
    return { provider, addReadHistory };
  }

  it('records read history after a successful launch', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);
    const { provider, addReadHistory } = makeProvider();

    await provider.openDesktopReader('book-1', 3);

    expect(openDesktopReaderMock).toHaveBeenCalledWith('book-1', 3);
    expect(addReadHistory).toHaveBeenCalledWith('book-1');
  });

  it('does not record and rethrows unchanged when the launch rejects', async () => {
    const launchError = new Error('reader_unsupported_platform');
    openDesktopReaderMock.mockRejectedValue(launchError);
    const { provider, addReadHistory } = makeProvider();

    await expect(provider.openDesktopReader('book-1')).rejects.toBe(launchError);
    // The in-app fallback in useReaderLaunch writes the entry instead; writing
    // here too would double-count it.
    expect(addReadHistory).not.toHaveBeenCalled();
  });

  it('still resolves when the read-history write rejects', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);
    const { provider, addReadHistory } = makeProvider();
    addReadHistory.mockRejectedValue(new Error('storage unavailable'));
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    // A failed write must not reject: useReaderLaunch would misread the launch
    // as failed, pop a toast and open a second in-app reader.
    await expect(provider.openDesktopReader('book-1')).resolves.toBeUndefined();
    expect(addReadHistory).toHaveBeenCalledWith('book-1');
    expect(warnSpy).toHaveBeenCalled();

    warnSpy.mockRestore();
  });

  it('still resolves when the read-history write throws synchronously', async () => {
    openDesktopReaderMock.mockResolvedValue(undefined);
    const { provider, addReadHistory } = makeProvider();
    // The real addReadHistory throws synchronously when no shelf is selected —
    // requireHistoryKey() runs before the promise is created — so a `.catch()`
    // on the call would never fire and the launch would wrongly reject.
    addReadHistory.mockImplementation(() => {
      throw new Error('No shelf selected.');
    });
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    await expect(provider.openDesktopReader('book-1')).resolves.toBeUndefined();
    expect(warnSpy).toHaveBeenCalled();

    warnSpy.mockRestore();
  });
});
