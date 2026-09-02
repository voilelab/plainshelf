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

// Stands in for the desktop binding: every name slugifies to `id`, and the
// default folder is the one the backend would build under its shelves dir.
function previewFor(id: string) {
  return vi.fn(() => Promise.resolve({ id, default_path: `/config/PlainShelf/shelves/${id}` }));
}

describe('useShelfManagement shelf-ID preview', () => {
  it('previews the id and the default folder as the name changes', async () => {
    provider.value = { previewDesktopShelfID: previewFor('shelf') };
    const { newShelfName, newShelfIDPreview, newShelfDefaultPath } = useShelfManagement();

    newShelfName.value = '小說';
    await nextTick();
    await vi.waitFor(() => expect(newShelfIDPreview.value).toBe('shelf'));
    expect(newShelfDefaultPath.value).toBe('/config/PlainShelf/shelves/shelf');
    expect(provider.value.previewDesktopShelfID).toHaveBeenCalledWith('小說');
  });

  it('clears the preview for an empty name without asking the backend', async () => {
    const previewDesktopShelfID = previewFor('unexpected');
    provider.value = { previewDesktopShelfID };
    const { newShelfName, newShelfIDPreview, newShelfDefaultPath } = useShelfManagement();

    newShelfName.value = '   ';
    await nextTick();
    await Promise.resolve();
    expect(newShelfIDPreview.value).toBe('');
    expect(newShelfDefaultPath.value).toBe('');
    expect(previewDesktopShelfID).not.toHaveBeenCalled();
  });

  it('leaves the preview empty when the provider cannot preview', async () => {
    provider.value = {};
    const { newShelfName, newShelfIDPreview, newShelfDefaultPath } = useShelfManagement();

    newShelfName.value = 'My Books';
    await nextTick();
    await Promise.resolve();
    expect(newShelfIDPreview.value).toBe('');
    expect(newShelfDefaultPath.value).toBe('');
  });
});

// The create form's two location branches. The default one asks the user for
// nothing but a name; the other adopts a folder they already have and is the
// only one that can be read-only.
describe('useShelfManagement add-shelf location branches', () => {
  it('creates the previewed folder, never read-only, on the default branch', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = { previewDesktopShelfID: previewFor('archive'), addDesktopShelf };
    const { newShelfName, newShelfLocationMode, newShelfReadOnly, canSubmitAddShelf, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Archive';
    await nextTick();
    await vi.waitFor(() => expect(canSubmitAddShelf.value).toBe(true));

    // Read-only cannot be reached from this branch in the UI; a value left over
    // from the other one must not survive the switch back either.
    expect(newShelfLocationMode.value).toBe('new');
    expect(newShelfReadOnly.value).toBe(false);

    await onSubmitAddShelf();
    expect(addDesktopShelf).toHaveBeenCalledWith(
      'Archive',
      '/config/PlainShelf/shelves/archive',
      '',
      false
    );
  });

  it('sends the read-only choice made on the existing-folder branch', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = { previewDesktopShelfID: previewFor('archive'), addDesktopShelf };
    const { newShelfName, newShelfLocationMode, newShelfDirectory, newShelfReadOnly, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Archive';
    newShelfLocationMode.value = 'existing';
    newShelfDirectory.value = '/mnt/archive';
    newShelfReadOnly.value = true;
    await nextTick();

    await onSubmitAddShelf();
    expect(addDesktopShelf).toHaveBeenCalledWith('Archive', '/mnt/archive', '', true);
  });

  it('resets read-only when the user goes back to the new-folder branch', async () => {
    provider.value = { previewDesktopShelfID: previewFor('archive') };
    const { newShelfLocationMode, newShelfReadOnly } = useShelfManagement();

    newShelfLocationMode.value = 'existing';
    newShelfReadOnly.value = true;
    await nextTick();

    newShelfLocationMode.value = 'new';
    await nextTick();
    expect(newShelfReadOnly.value).toBe(false);
  });

  // Without this the submit reaches Go and comes back as
  // `shelf directory must be an absolute path` (desktop/shelves.go).
  it('refuses a relative folder before it reaches the backend', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = { previewDesktopShelfID: previewFor('archive'), addDesktopShelf };
    const {
      newShelfName,
      newShelfLocationMode,
      newShelfDirectory,
      newShelfDirectoryError,
      canSubmitAddShelf,
      onSubmitAddShelf
    } = useShelfManagement();

    newShelfName.value = 'Archive';
    newShelfLocationMode.value = 'existing';
    newShelfDirectory.value = 'books/archive';
    await nextTick();

    expect(newShelfDirectoryError.value).not.toBe('');
    expect(canSubmitAddShelf.value).toBe(false);
    await onSubmitAddShelf();
    expect(addDesktopShelf).not.toHaveBeenCalled();

    newShelfDirectory.value = '/mnt/archive';
    await nextTick();
    expect(newShelfDirectoryError.value).toBe('');
    expect(canSubmitAddShelf.value).toBe(true);
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
  it('clears the read-only toggle and the location branch when the form is reopened', async () => {
    provider.value = { addDesktopShelf: vi.fn(() => Promise.resolve()) };
    const { newShelfLocationMode, newShelfDirectory, newShelfReadOnly, openAddShelfModal } =
      useShelfManagement();

    newShelfLocationMode.value = 'existing';
    newShelfDirectory.value = '/mnt/archive';
    newShelfReadOnly.value = true;
    openAddShelfModal();

    expect(newShelfLocationMode.value).toBe('new');
    expect(newShelfDirectory.value).toBe('');
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
