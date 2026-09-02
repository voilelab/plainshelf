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

/**
 * The store is module state, so each case needs its own copy of the module.
 */
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

  // The layout and the Shelves panel both want the list on a settings page load,
  // and a child's onMounted runs before its parent's. Without a shared load the
  // request count came down to which round-trip finished first.
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

  // A shelf that was just added or removed has to be re-read, so the explicit
  // refresh must not be short-circuited by an earlier load.
  it('fetchShelves still refreshes an already loaded list', async () => {
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    await store.fetchShelves();

    expect(listShelves).toHaveBeenCalledTimes(2);
  });

  // A failed load must not be remembered as done, or the list would stay empty
  // until the page is reloaded.
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

// A read-only shelf and a writable one live in the same list, so the flag has
// to follow the selection rather than the list as a whole. This is what every
// write affordance in the app ends up asking, through useWriteAccess.
describe('activeShelfReadOnly', () => {
  it('follows the selected shelf when both kinds are listed', async () => {
    listShelves.mockResolvedValue([
      { id: 'shelf-1', name: 'Writable', readOnly: false },
      { id: 'shelf-2', name: 'Archive', readOnly: true }
    ]);
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    expect(store.selectedShelfID.value).toBe('shelf-1');
    expect(store.activeShelfReadOnly.value).toBe(false);

    store.selectShelf('shelf-2');
    expect(store.activeShelfReadOnly.value).toBe(true);

    store.selectShelf('shelf-1');
    expect(store.activeShelfReadOnly.value).toBe(false);
  });

  // Before the list arrives there is nothing to read the flag off. Defaulting
  // to writable keeps the buttons on screen for the shelf that accepts writes;
  // the server still refuses the read-only one with 409.
  it('reports writable before the list loads', async () => {
    const store = await freshStore();

    expect(store.activeShelfReadOnly.value).toBe(false);
  });

  // The offline fallback selects a persisted id the list does not contain.
  it('reports writable for a shelf the list does not describe', async () => {
    listShelves.mockResolvedValue([{ id: 'shelf-1', name: 'Writable', readOnly: false }]);
    const store = await freshStore();

    await store.ensureShelvesLoaded();
    store.selectShelf('unknown-shelf');

    expect(store.activeShelfReadOnly.value).toBe(false);
  });
});
