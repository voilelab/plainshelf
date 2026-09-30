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
      previewDesktopShelfID: vi.fn((name: string) =>
        Promise.resolve(
          name === '小說'
            ? { id: 'shelf', defaultPath: '/config/PlainShelf/shelves/shelf' }
            : { id: `id-${name}`, defaultPath: `/config/PlainShelf/shelves/id-${name}` }
        )
      )
    };
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = '小說';
    await nextTick();
    await vi.waitFor(() => expect(newShelfIDPreview.value).toBe('shelf'));
    expect(provider.value.previewDesktopShelfID).toHaveBeenCalledWith('小說');
  });

  it('clears the preview for an empty name without asking the backend', async () => {
    const previewDesktopShelfID = vi.fn(() =>
      Promise.resolve({ id: 'unexpected', defaultPath: '/unexpected' })
    );
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

describe('useShelfManagement default shelf directory', () => {
  function previewing(defaultPath: string) {
    return vi.fn(() => Promise.resolve({ id: 'novels', defaultPath }));
  }

  // The point of the default: a shelf can be created from a name alone, and the
  // path the dialog showed is the path that is written — the backend does not
  // derive it a second time.
  it('submits the previewed default path when the user picks no directory', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = {
      previewDesktopShelfID: previewing('/config/PlainShelf/shelves/novels'),
      addDesktopShelf
    };
    const { newShelfName, newShelfEffectiveDirectory, canSubmitAddShelf, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Novels';
    await nextTick();
    await vi.waitFor(() => expect(canSubmitAddShelf.value).toBe(true));
    expect(newShelfEffectiveDirectory.value).toBe('/config/PlainShelf/shelves/novels');

    await onSubmitAddShelf();

    expect(addDesktopShelf).toHaveBeenCalledWith({
      name: 'Novels',
      libRoot: '/config/PlainShelf/shelves/novels',
      scanInterval: '',
      bookCheckInterval: '',
      readOnly: false
    });
  });

  it('keeps a directory the user chose when the name changes afterwards', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = {
      previewDesktopShelfID: previewing('/config/PlainShelf/shelves/novels'),
      addDesktopShelf
    };
    const {
      newShelfName,
      newShelfLocationMode,
      newShelfDirectory,
      newShelfEffectiveDirectory,
      onSubmitAddShelf
    } = useShelfManagement();

    newShelfLocationMode.value = 'existing';
    newShelfDirectory.value = '/mnt/books';
    newShelfName.value = 'Novels';
    await nextTick();
    await vi.waitFor(() => expect(newShelfEffectiveDirectory.value).toBe('/mnt/books'));

    await onSubmitAddShelf();

    expect(addDesktopShelf).toHaveBeenCalledWith(
      expect.objectContaining({ libRoot: '/mnt/books' })
    );
  });

  // The default path shares the preview's latest-wins token, so a slow response
  // cannot repopulate the directory of a form that has since been cleared.
  it('drops a late default path after the form is reset', async () => {
    let resolveLate: ((value: { id: string; defaultPath: string }) => void) | undefined;
    provider.value = {
      previewDesktopShelfID: vi.fn(
        () =>
          new Promise<{ id: string; defaultPath: string }>((resolve) => {
            resolveLate = resolve;
          })
      )
    };
    const { newShelfName, newShelfIDPreview, newShelfEffectiveDirectory, openAddShelfModal } =
      useShelfManagement();

    newShelfName.value = 'Novels';
    await nextTick();
    openAddShelfModal();
    resolveLate?.({ id: 'novels', defaultPath: '/config/PlainShelf/shelves/novels' });
    await Promise.resolve();
    await Promise.resolve();

    expect(newShelfIDPreview.value).toBe('');
    expect(newShelfEffectiveDirectory.value).toBe('');
  });

  // The default belongs to the name it was derived from. Holding the previous
  // answer across the next lookup would let a submit in that window create a
  // shelf named for the new name under the old name's directory.
  it('drops the previous default while the next name is being previewed', async () => {
    let pending: ((value: { id: string; defaultPath: string }) => void) | undefined;
    provider.value = {
      previewDesktopShelfID: vi.fn((name: string) => {
        if (name === 'Novels') {
          return Promise.resolve({ id: 'novels', defaultPath: '/shelves/novels' });
        }
        return new Promise<{ id: string; defaultPath: string }>((resolve) => {
          pending = resolve;
        });
      }),
      addDesktopShelf: vi.fn(() => Promise.resolve())
    };
    const { newShelfName, newShelfIDPreview, newShelfEffectiveDirectory, canSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Novels';
    await nextTick();
    await vi.waitFor(() => expect(newShelfEffectiveDirectory.value).toBe('/shelves/novels'));

    newShelfName.value = 'Poetry';
    await nextTick();
    expect(newShelfEffectiveDirectory.value).toBe('');
    expect(newShelfIDPreview.value).toBe('');
    expect(canSubmitAddShelf.value).toBe(false);

    pending?.({ id: 'poetry', defaultPath: '/shelves/poetry' });
    await vi.waitFor(() => expect(newShelfEffectiveDirectory.value).toBe('/shelves/poetry'));
  });

  // shelf.NewShelf refuses to create lib_root for a read-only shelf, and the
  // default names a directory that does not exist yet. The branch is what keeps
  // the two apart now: read-only is only reachable where the user supplies a
  // directory that already exists.
  it('never pairs the default directory with read-only', async () => {
    provider.value = {
      previewDesktopShelfID: previewing('/config/PlainShelf/shelves/novels'),
      addDesktopShelf: vi.fn(() => Promise.resolve())
    };
    const {
      newShelfName,
      newShelfLocationMode,
      newShelfReadOnly,
      newShelfDirectory,
      newShelfEffectiveDirectory
    } = useShelfManagement();

    newShelfName.value = 'Novels';
    newShelfLocationMode.value = 'existing';
    newShelfReadOnly.value = true;
    newShelfDirectory.value = '/mnt/archive';
    await nextTick();
    expect(newShelfEffectiveDirectory.value).toBe('/mnt/archive');

    // Going back to the default branch drops the read-only choice with it.
    newShelfLocationMode.value = 'new';
    await nextTick();
    expect(newShelfReadOnly.value).toBe(false);
    await vi.waitFor(() =>
      expect(newShelfEffectiveDirectory.value).toBe('/config/PlainShelf/shelves/novels')
    );
  });

  it('cannot submit off the desktop, where there is no default to fall back on', async () => {
    provider.value = { addDesktopShelf: vi.fn(() => Promise.resolve()) };
    const { newShelfName, canSubmitAddShelf } = useShelfManagement();

    newShelfName.value = 'Novels';
    await nextTick();
    await Promise.resolve();
    expect(canSubmitAddShelf.value).toBe(false);
  });

  // Neither interval is asked for while creating a shelf; both keep the backend
  // default and stay adjustable in the modify dialog.
  it('creates a shelf on the default branch with no intervals and read-only off', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = {
      previewDesktopShelfID: previewing('/config/PlainShelf/shelves/novels'),
      addDesktopShelf
    };
    const { newShelfName, newShelfLocationMode, canSubmitAddShelf, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Novels';
    expect(newShelfLocationMode.value).toBe('new');
    await nextTick();
    await vi.waitFor(() => expect(canSubmitAddShelf.value).toBe(true));

    await onSubmitAddShelf();
    expect(addDesktopShelf).toHaveBeenCalledWith({
      name: 'Novels',
      libRoot: '/config/PlainShelf/shelves/novels',
      scanInterval: '',
      bookCheckInterval: '',
      readOnly: false
    });
  });

  it('sends the read-only choice made on the existing-folder branch', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = {
      previewDesktopShelfID: previewing('/config/PlainShelf/shelves/novels'),
      addDesktopShelf
    };
    const { newShelfName, newShelfLocationMode, newShelfDirectory, newShelfReadOnly, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Archive';
    newShelfLocationMode.value = 'existing';
    newShelfDirectory.value = '/mnt/archive';
    newShelfReadOnly.value = true;
    await nextTick();

    await onSubmitAddShelf();
    expect(addDesktopShelf).toHaveBeenCalledWith(
      expect.objectContaining({ libRoot: '/mnt/archive', readOnly: true })
    );
  });

  // Without this the submit reaches Go and comes back as
  // `shelf directory must be an absolute path` (desktop/shelves.go).
  it('refuses a relative folder before it reaches the backend', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = {
      previewDesktopShelfID: previewing('/config/PlainShelf/shelves/novels'),
      addDesktopShelf
    };
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
  it('creates a shelf with the read-only toggle as it was left', async () => {
    const addDesktopShelf = vi.fn(() => Promise.resolve());
    provider.value = { addDesktopShelf };
    const { newShelfName, newShelfLocationMode, newShelfDirectory, newShelfReadOnly, onSubmitAddShelf } =
      useShelfManagement();

    newShelfName.value = 'Archive';
    newShelfLocationMode.value = 'existing';
    newShelfDirectory.value = '/mnt/archive';
    newShelfReadOnly.value = true;
    await nextTick();
    await onSubmitAddShelf();

    expect(addDesktopShelf).toHaveBeenCalledWith({
      name: 'Archive',
      libRoot: '/mnt/archive',
      scanInterval: '',
      bookCheckInterval: '',
      readOnly: true
    });
  });

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
        Promise.resolve({
          id: 'archive',
          name: 'Archive',
          path: '/mnt/archive',
          scan_interval: '10m',
          book_check_interval: '5m',
          read_only: true
        })
      ),
      modifyDesktopShelf
    };
    const {
      requestModifyShelf,
      modifyShelfReadOnly,
      modifyShelfBookCheckInterval,
      onSubmitModifyShelf
    } = useShelfManagement();

    await requestModifyShelf({ id: 'archive', name: 'Archive' });
    expect(modifyShelfReadOnly.value).toBe(true);
    // book_check_interval is loaded from the stored details so it round-trips
    // rather than resetting to the default on an unrelated save.
    expect(modifyShelfBookCheckInterval.value).toBe('5m');

    modifyShelfReadOnly.value = false;
    await onSubmitModifyShelf();

    expect(modifyDesktopShelf).toHaveBeenCalledWith({
      shelfID: 'archive',
      name: 'Archive',
      scanInterval: '10m',
      bookCheckInterval: '5m',
      readOnly: false
    });
  });
});
