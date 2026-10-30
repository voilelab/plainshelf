import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const listFolders = vi.fn();
const listFolderMeta = vi.fn();
let withFolderMeta = true;

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => (withFolderMeta ? { listFolders, listFolderMeta } : { listFolders })
}));

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
  listFolderMeta.mockReset();
  listFolderMeta.mockResolvedValue({ icons: [], folders: {} });
  withFolderMeta = true;
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
    await (fetchFolders as (...args: unknown[]) => Promise<void>)(new Event('click'));
    expect(listFolders).toHaveBeenCalledTimes(1);
  });
});

describe('useFolderStore folder metadata', () => {
  it('loads each folder\'s metadata beside the folder list', async () => {
    const { folders, folderMeta, fetchFolders } = await freshStore();
    listFolders.mockResolvedValueOnce(['/', 'Fiction']);
    const meta = { Fiction: { icon: 'rocket', version: 'v1', readOnly: false } };
    listFolderMeta.mockResolvedValueOnce({ icons: ['rocket'], folders: meta });

    await fetchFolders();

    expect(folders.value).toEqual(['/', 'Fiction']);
    expect(folderMeta.value).toEqual(meta);
  });

  it('keeps the folder list when the metadata cannot be read', async () => {
    const { folders, folderMeta, error, fetchFolders } = await freshStore();
    listFolders.mockResolvedValueOnce(['/', 'Fiction']);
    listFolderMeta.mockRejectedValueOnce(new Error('offline'));

    await fetchFolders();

    expect(folders.value).toEqual(['/', 'Fiction']);
    expect(folderMeta.value).toEqual({});
    expect(error.value).toBe('');
  });

  it('shows no metadata on a backend without the listing', async () => {
    withFolderMeta = false;
    const { folderMeta, fetchFolders, fetchFolderMeta } = await freshStore();
    listFolders.mockResolvedValueOnce(['/']);

    await fetchFolders();
    await fetchFolderMeta();

    expect(folderMeta.value).toEqual({});
    expect(listFolderMeta).not.toHaveBeenCalled();
  });
});
