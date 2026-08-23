import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const listFolders = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listFolders })
}));

// The store is a module-level singleton, so each test needs its own copy of it.
// The reset also gives the store a fresh `@/api/client`, so the errors it is
// fed have to come from that same copy or its `instanceof ApiError` check fails.
async function freshStore() {
  vi.resetModules();
  const { ApiError } = await import('@/api/client');
  const { useFolderStore } = await import('./useFolderStore');
  return {
    ...useFolderStore(),
    initializing: () => new ApiError('shelf is initializing', { status: 503 })
  };
}

beforeEach(() => {
  listFolders.mockReset();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useFolderStore', () => {
  it('keeps retrying while the shelf reports it is initializing', async () => {
    const { folders, loading, error, loaded, fetchFolders, initializing } = await freshStore();
    listFolders
      .mockRejectedValueOnce(initializing())
      .mockRejectedValueOnce(initializing())
      .mockResolvedValueOnce(['/', '/fiction']);

    await fetchFolders();

    // Between attempts the sidebar stays in its loading state instead of
    // showing an error the shelf resolves on its own.
    expect(loading.value).toBe(true);
    expect(error.value).toBe('');
    expect(loaded.value).toBe(false);

    await vi.advanceTimersByTimeAsync(3000);
    expect(loading.value).toBe(true);

    await vi.advanceTimersByTimeAsync(3000);
    expect(listFolders).toHaveBeenCalledTimes(3);
    expect(folders.value).toEqual(['/', '/fiction']);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');
    expect(loaded.value).toBe(true);
  });

  it('gives up with an error after the auto-retry budget is spent', async () => {
    const { loading, error, loaded, fetchFolders, initializing } = await freshStore();
    listFolders.mockRejectedValue(initializing());

    await fetchFolders();
    await vi.advanceTimersByTimeAsync(3000 * 10);

    expect(listFolders).toHaveBeenCalledTimes(10);
    expect(loading.value).toBe(false);
    expect(loaded.value).toBe(false);
    expect(error.value).not.toBe('');
  });

  it('reports any other failure without retrying', async () => {
    listFolders.mockRejectedValue(new Error('boom'));

    const { loading, error, fetchFolders } = await freshStore();
    await fetchFolders();

    expect(listFolders).toHaveBeenCalledTimes(1);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('boom');
  });

  it('restarts the retry budget when the user retries by hand', async () => {
    const { fetchFolders, initializing } = await freshStore();
    listFolders.mockRejectedValue(initializing());

    await fetchFolders();
    await vi.advanceTimersByTimeAsync(3000 * 10);
    expect(listFolders).toHaveBeenCalledTimes(10);

    listFolders.mockReset();
    listFolders.mockResolvedValue(['/']);
    // Bound directly to the Retry button, so it is called with a click event.
    await (fetchFolders as (...args: unknown[]) => Promise<void>)(new Event('click'));
    expect(listFolders).toHaveBeenCalledTimes(1);
  });
});
