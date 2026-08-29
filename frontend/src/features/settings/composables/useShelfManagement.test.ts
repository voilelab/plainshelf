// @vitest-environment jsdom
import { nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The composable resolves the shelf-ID preview and the reveal-in-Finder action
// through the active bookshelf provider; hold a swappable provider so each case
// controls what those bindings do (or whether they exist at all).
const provider = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
vi.mock('@/providers', () => ({ getBookshelfProvider: () => provider.value }));

import { useShelfManagement } from './useShelfManagement';
import { setLocale } from '@/i18n';

beforeEach(() => {
  setLocale('en');
  provider.value = {};
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('useShelfManagement shelf-ID preview', () => {
  it('previews the id the backend would assign as the name changes', async () => {
    provider.value = {
      previewDesktopShelfID: vi.fn((name: string) => Promise.resolve(name === '小說' ? 'shelf' : `id-${name}`))
    };
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = '小說';
    await nextTick();
    await vi.waitFor(() => expect(newShelfIDPreview.value).toBe('shelf'));
    expect(provider.value.previewDesktopShelfID).toHaveBeenCalledWith('小說');
  });

  it('clears the preview for an empty name without asking the backend', async () => {
    const previewDesktopShelfID = vi.fn(() => Promise.resolve('unexpected'));
    provider.value = { previewDesktopShelfID };
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = '   ';
    await nextTick();
    await Promise.resolve();
    expect(newShelfIDPreview.value).toBe('');
    expect(previewDesktopShelfID).not.toHaveBeenCalled();
  });

  it('leaves the preview empty when the provider cannot preview', async () => {
    provider.value = {};
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = 'My Books';
    await nextTick();
    await Promise.resolve();
    expect(newShelfIDPreview.value).toBe('');
  });
});

describe('useShelfManagement openShelfFolder', () => {
  it('asks the provider to reveal the shelf folder', async () => {
    const openDesktopShelfFolder = vi.fn(() => Promise.resolve());
    provider.value = { openDesktopShelfFolder };
    const { openShelfFolder, shelfOpError } = useShelfManagement();

    await openShelfFolder('my-books');
    expect(openDesktopShelfFolder).toHaveBeenCalledWith('my-books');
    expect(shelfOpError.value).toBe('');
  });

  it('surfaces a reveal failure on the panel', async () => {
    provider.value = {
      openDesktopShelfFolder: vi.fn(() => Promise.reject(new Error('boom')))
    };
    const { openShelfFolder, shelfOpError } = useShelfManagement();

    await openShelfFolder('my-books');
    expect(shelfOpError.value).toBe('boom');
  });

  it('is a no-op when the provider cannot reveal folders', async () => {
    provider.value = {};
    const { openShelfFolder, shelfOpError } = useShelfManagement();

    await openShelfFolder('my-books');
    expect(shelfOpError.value).toBe('');
  });
});

describe('useShelfManagement read-only shelves', () => {
  it('creates a shelf with the read-only toggle as it was left', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = { addDesktopShelf };
    const { newShelfName, newShelfDirectory, newShelfReadOnly, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Archive';
    newShelfDirectory.value = '/mnt/archive';
    newShelfReadOnly.value = true;
    await onSubmitAddShelf();

    expect(addDesktopShelf).toHaveBeenCalledWith('Archive', '/mnt/archive', '', true);
  });

  it('clears the read-only toggle when the add-shelf form is reopened', async () => {
    provider.value = { addDesktopShelf: vi.fn(() => Promise.resolve()) };
    const { newShelfReadOnly, openAddShelfModal } = useShelfManagement();

    newShelfReadOnly.value = true;
    openAddShelfModal();

    expect(newShelfReadOnly.value).toBe(false);
  });

  // Loading the stored value is what keeps read-only reversible from the UI: a
  // form that always opened unchecked would silently turn the shelf writable on
  // an unrelated save, and one that could not send `false` would be a one-way
  // door. Both directions have to reach the backend.
  it('loads a shelf read-only state into the modify form and sends the change back', async () => {
    const modifyDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = {
      getDesktopShelfDetails: vi.fn(() =>
        Promise.resolve({ id: 'archive', name: 'Archive', path: '/mnt/archive', scan_interval: '10m', read_only: true })
      ),
      modifyDesktopShelf
    };
    const { requestModifyShelf, modifyShelfReadOnly, onSubmitModifyShelf } = useShelfManagement();

    await requestModifyShelf({ id: 'archive', name: 'Archive' });
    expect(modifyShelfReadOnly.value).toBe(true);

    modifyShelfReadOnly.value = false;
    await onSubmitModifyShelf();

    expect(modifyDesktopShelf).toHaveBeenCalledWith('archive', 'Archive', '10m', false);
  });
});
