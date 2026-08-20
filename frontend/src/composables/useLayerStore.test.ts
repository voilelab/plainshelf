import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const listLayers = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listLayers })
}));

// The store is a module-level singleton, so each test needs its own copy of it.
// The reset also gives the store a fresh `@/api/client`, so the errors it is
// fed have to come from that same copy or its `instanceof ApiError` check fails.
async function freshStore() {
  vi.resetModules();
  const { ApiError } = await import('@/api/client');
  const { useLayerStore } = await import('./useLayerStore');
  return {
    ...useLayerStore(),
    initializing: () => new ApiError('shelf is initializing', { status: 503 })
  };
}

beforeEach(() => {
  listLayers.mockReset();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useLayerStore', () => {
  it('keeps retrying while the shelf reports it is initializing', async () => {
    const { layers, loading, error, loaded, fetchLayers, initializing } = await freshStore();
    listLayers
      .mockRejectedValueOnce(initializing())
      .mockRejectedValueOnce(initializing())
      .mockResolvedValueOnce(['/', '/fiction']);

    await fetchLayers();

    // Between attempts the sidebar stays in its loading state instead of
    // showing an error the shelf resolves on its own.
    expect(loading.value).toBe(true);
    expect(error.value).toBe('');
    expect(loaded.value).toBe(false);

    await vi.advanceTimersByTimeAsync(3000);
    expect(loading.value).toBe(true);

    await vi.advanceTimersByTimeAsync(3000);
    expect(listLayers).toHaveBeenCalledTimes(3);
    expect(layers.value).toEqual(['/', '/fiction']);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');
    expect(loaded.value).toBe(true);
  });

  it('gives up with an error after the auto-retry budget is spent', async () => {
    const { loading, error, loaded, fetchLayers, initializing } = await freshStore();
    listLayers.mockRejectedValue(initializing());

    await fetchLayers();
    await vi.advanceTimersByTimeAsync(3000 * 10);

    expect(listLayers).toHaveBeenCalledTimes(10);
    expect(loading.value).toBe(false);
    expect(loaded.value).toBe(false);
    expect(error.value).not.toBe('');
  });

  it('reports any other failure without retrying', async () => {
    listLayers.mockRejectedValue(new Error('boom'));

    const { loading, error, fetchLayers } = await freshStore();
    await fetchLayers();

    expect(listLayers).toHaveBeenCalledTimes(1);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('boom');
  });

  it('restarts the retry budget when the user retries by hand', async () => {
    const { fetchLayers, initializing } = await freshStore();
    listLayers.mockRejectedValue(initializing());

    await fetchLayers();
    await vi.advanceTimersByTimeAsync(3000 * 10);
    expect(listLayers).toHaveBeenCalledTimes(10);

    listLayers.mockReset();
    listLayers.mockResolvedValue(['/']);
    // Bound directly to the Retry button, so it is called with a click event.
    await (fetchLayers as (...args: unknown[]) => Promise<void>)(new Event('click'));
    expect(listLayers).toHaveBeenCalledTimes(1);
  });
});
