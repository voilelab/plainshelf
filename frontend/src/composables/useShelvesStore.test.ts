import { beforeEach, describe, expect, it, vi } from 'vitest';

const listShelves = vi.fn();
const ensureActiveShelf = vi.fn(() => 'shelf-1');

vi.mock('@/api/shelves', () => ({
  listShelves: () => listShelves(),
  ensureActiveShelf: () => ensureActiveShelf()
}));
vi.mock('@/api/client', () => ({
  ApiError: class ApiError extends Error {
    status = 0;
  },
  setActiveShelfID: vi.fn()
}));
vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ getPersistedShelfID: () => '' })
}));
vi.mock('@/i18n', () => ({ t: (key: string) => key }));

async function freshStore() {
  vi.resetModules();
  const { useShelvesStore } = await import('./useShelvesStore');
  return useShelvesStore();
}

describe('useShelvesStore', () => {
  beforeEach(() => {
    listShelves.mockReset();
    listShelves.mockResolvedValue([{ id: 'shelf-1', name: 'Shelf 1' }]);
  });

  it('fetches once when two callers ask before the request settles', async () => {
    const store = await freshStore();

    await Promise.all([store.ensureShelvesLoaded(), store.ensureShelvesLoaded()]);

    expect(listShelves).toHaveBeenCalledTimes(1);
    expect(store.loaded.value).toBe(true);
  });

  it('does not fetch again once the list is loaded', async () => {
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    await store.ensureShelvesLoaded();

    expect(listShelves).toHaveBeenCalledTimes(1);
  });

  it('fetchShelves still refreshes an already loaded list', async () => {
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    await store.fetchShelves();

    expect(listShelves).toHaveBeenCalledTimes(2);
  });

  it('retries after a failed load', async () => {
    const store = await freshStore();
    listShelves.mockRejectedValueOnce(new Error('offline'));

    await store.ensureShelvesLoaded();
    expect(store.loaded.value).toBe(false);

    await store.ensureShelvesLoaded();
    expect(listShelves).toHaveBeenCalledTimes(2);
    expect(store.loaded.value).toBe(true);
  });
});

describe('selectedShelfReadOnly', () => {
  it('follows the selected shelf when both kinds are listed', async () => {
    listShelves.mockResolvedValue([
      { id: 'shelf-1', name: 'Writable', readOnly: false },
      { id: 'shelf-2', name: 'Archive', readOnly: true }
    ]);
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    expect(store.selectedShelfID.value).toBe('shelf-1');
    expect(store.selectedShelfReadOnly.value).toBe(false);

    store.selectShelf('shelf-2');
    expect(store.selectedShelfReadOnly.value).toBe(true);

    store.selectShelf('shelf-1');
    expect(store.selectedShelfReadOnly.value).toBe(false);
  });

  it('reports writable before the list loads', async () => {
    const store = await freshStore();

    expect(store.selectedShelfReadOnly.value).toBe(false);
  });

  it('reports writable for a shelf the list does not describe', async () => {
    listShelves.mockResolvedValue([{ id: 'shelf-1', name: 'Writable', readOnly: false }]);
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    store.selectShelf('unknown-shelf');

    expect(store.selectedShelfReadOnly.value).toBe(false);
  });
});

describe('transferDestinationShelves', () => {
  it('offers only the writable shelves that are not the selected one', async () => {
    listShelves.mockResolvedValue([
      { id: 'shelf-1', name: 'Shelf 1', readOnly: false },
      { id: 'shelf-2', name: 'Shelf 2', readOnly: false },
      { id: 'archive', name: 'Archive', readOnly: true }
    ]);
    const store = await freshStore();

    await store.ensureShelvesLoaded();

    expect(store.selectedShelfID.value).toBe('shelf-1');
    expect(store.transferDestinationShelves.value.map((shelf) => shelf.id)).toEqual(['shelf-2']);
  });

  it('still offers destinations when the selected shelf is read-only', async () => {
    listShelves.mockResolvedValue([
      { id: 'shelf-1', name: 'Archive', readOnly: true },
      { id: 'shelf-2', name: 'Shelf 2', readOnly: false }
    ]);
    const store = await freshStore();

    await store.ensureShelvesLoaded();

    expect(store.transferDestinationShelves.value.map((shelf) => shelf.id)).toEqual(['shelf-2']);
  });
});
