import { computed, ref, watch } from 'vue';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { getBookshelfProvider } from '@/providers';
import { useI18n } from '@/i18n';
import { isAbsoluteShelfPath } from '@/features/settings/utils/shelfPath';

interface ShelfRef {
  id: string;
  name: string;
}

/** Where a new shelf's folder comes from: created by PlainShelf, or adopted. */
export type ShelfLocationMode = 'new' | 'existing';

/**
 * The settings page's shelf table: adding, modifying and removing a shelf, plus
 * the form and busy/error state each of those modals reads. Shelf data itself
 * still comes from the shared shelves store, so a change here is reflected
 * anywhere else that store is mounted.
 *
 * Every operation goes through the bookshelf provider and is a no-op when the
 * running shell does not expose it, which is how the server build keeps its
 * shelves read-only.
 */
export function useShelfManagement() {
  const { t } = useI18n();
  const { shelves, loading: shelvesLoading, error: shelvesError, fetchShelves, ensureShelvesLoaded } = useShelvesStore();

  const shelfOpError = ref('');

  const removingShelfIDs = ref<Set<string>>(new Set());
  const pendingRemoveShelf = ref<ShelfRef | null>(null);
  const shelfRemoveModalError = ref('');

  const showAddShelfModal = ref(false);
  const newShelfName = ref('');
  // Which of the two ways to place a new shelf the user has chosen. The default
  // branch creates a folder PlainShelf owns and needs nothing but a name; the
  // other one adopts a folder the user already has. They share this one set of
  // refs rather than two parallel forms, so there is a single place to reset.
  const newShelfLocationMode = ref<ShelfLocationMode>('new');
  const newShelfDirectory = ref('');
  const newShelfReadOnly = ref(false);
  const addingShelf = ref(false);
  const addShelfError = ref('');

  // Read-only is a property of a folder that already exists — it turns off the
  // directory creation the default branch is entirely built on — so leaving the
  // existing-folder branch has to put it back, not just stop showing it.
  watch(newShelfLocationMode, (mode) => {
    if (mode === 'new') {
      newShelfReadOnly.value = false;
    }
  });

  // What creating a shelf under the typed name would produce, shown live: the
  // id so a name that slugifies to nothing (e.g. a purely non-ASCII "小說")
  // visibly becomes "shelf" before the id is created and frozen as the
  // reading-progress key, and the folder the default branch would create. Both
  // empty when there is no preview: off the desktop, or for an empty name.
  const newShelfIDPreview = ref('');
  const newShelfDefaultPath = ref('');
  // Latest-wins guard: the async preview lags keystrokes, so a slow earlier
  // response must not overwrite a newer one (or a reset).
  let shelfIDPreviewToken = 0;

  function clearShelfNamePreview(): void {
    newShelfIDPreview.value = '';
    newShelfDefaultPath.value = '';
  }

  async function refreshShelfIDPreview(name: string): Promise<void> {
    const token = ++shelfIDPreviewToken;
    const provider = getBookshelfProvider();
    if (!provider.previewDesktopShelfID || name.trim().length === 0) {
      clearShelfNamePreview();
      return;
    }
    try {
      const preview = await provider.previewDesktopShelfID(name.trim());
      if (token === shelfIDPreviewToken) {
        newShelfIDPreview.value = preview.id;
        newShelfDefaultPath.value = preview.default_path;
      }
    } catch {
      if (token === shelfIDPreviewToken) {
        clearShelfNamePreview();
      }
    }
  }

  // The directory the form would submit: the previewed default on the new-folder
  // branch, and what the user typed or browsed to on the other one.
  const newShelfResolvedDirectory = computed(() =>
    newShelfLocationMode.value === 'new'
      ? newShelfDefaultPath.value
      : newShelfDirectory.value.trim()
  );

  // A relative path is refused here rather than by Go, which would answer the
  // finished submit with `shelf directory must be an absolute path`. Only shown
  // once something has been typed, so an untouched field is not an error.
  const newShelfDirectoryError = computed(() => {
    if (newShelfLocationMode.value !== 'existing') {
      return '';
    }
    const dir = newShelfDirectory.value.trim();
    if (dir === '' || isAbsoluteShelfPath(dir)) {
      return '';
    }
    return t('settings.shelves.addShelfDirectoryNotAbsolute');
  });

  const canSubmitAddShelf = computed(() => {
    if (newShelfName.value.trim().length === 0) {
      return false;
    }
    const dir = newShelfResolvedDirectory.value;
    return dir.length > 0 && isAbsoluteShelfPath(dir);
  });

  watch(newShelfName, (name) => {
    void refreshShelfIDPreview(name);
  });

  const pendingModifyShelf = ref<ShelfRef | null>(null);
  const showModifyShelfModal = ref(false);
  const modifyShelfName = ref('');
  const modifyShelfScanInterval = ref('');
  const modifyShelfReadOnly = ref(false);
  const modifyShelfPath = ref('');
  const modifyingShelf = ref(false);
  const modifyShelfError = ref('');
  const canSubmitModifyShelf = computed(() => modifyShelfName.value.trim().length > 0);

  function requestRemoveShelf(shelf: ShelfRef): void {
    shelfOpError.value = '';
    shelfRemoveModalError.value = '';
    pendingRemoveShelf.value = shelf;
  }

  function cancelRemoveShelf(): void {
    const shelf = pendingRemoveShelf.value;
    if (shelf && removingShelfIDs.value.has(shelf.id)) {
      return;
    }

    pendingRemoveShelf.value = null;
    shelfRemoveModalError.value = '';
  }

  async function confirmRemoveShelf(): Promise<void> {
    const shelf = pendingRemoveShelf.value;
    if (!shelf) {
      return;
    }
    const provider = getBookshelfProvider();
    if (!provider.removeDesktopShelf) {
      return;
    }

    removingShelfIDs.value = new Set([...removingShelfIDs.value, shelf.id]);

    shelfOpError.value = '';
    try {
      await provider.removeDesktopShelf(shelf.id);
      await fetchShelves();
    } catch (err) {
      shelfOpError.value = err instanceof Error ? err.message : t('settings.shelves.removeFailed');
    } finally {
      const next = new Set(removingShelfIDs.value);
      next.delete(shelf.id);
      removingShelfIDs.value = next;
      if (!shelfOpError.value) {
        pendingRemoveShelf.value = null;
        shelfRemoveModalError.value = '';
      } else {
        shelfRemoveModalError.value = shelfOpError.value;
      }
    }
  }

  function resetAddShelfForm(): void {
    newShelfName.value = '';
    newShelfLocationMode.value = 'new';
    newShelfDirectory.value = '';
    newShelfReadOnly.value = false;
    addShelfError.value = '';
    // Invalidate any in-flight preview so its late response cannot repopulate
    // the fields after the form is cleared.
    shelfIDPreviewToken++;
    clearShelfNamePreview();
  }

  // Reveals a shelf's lib_root in the host file explorer (desktop only); a
  // no-op elsewhere. Errors surface on the panel like the other shelf ops.
  async function openShelfFolder(shelfID: string): Promise<void> {
    const provider = getBookshelfProvider();
    if (!provider.openDesktopShelfFolder) {
      return;
    }
    shelfOpError.value = '';
    try {
      await provider.openDesktopShelfFolder(shelfID);
    } catch (err) {
      shelfOpError.value =
        err instanceof Error ? err.message : t('settings.shelves.openFolderFailed');
    }
  }

  function openAddShelfModal(): void {
    resetAddShelfForm();
    showAddShelfModal.value = true;
  }

  function closeAddShelfModal(): void {
    if (addingShelf.value) {
      return;
    }

    showAddShelfModal.value = false;
    resetAddShelfForm();
  }

  async function onBrowseShelfDirectory(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!provider.openDesktopShelfDirectory) {
      return;
    }
    const dir = await provider.openDesktopShelfDirectory();
    if (dir) {
      newShelfDirectory.value = dir;
    }
  }

  async function onSubmitAddShelf(): Promise<void> {
    const name = newShelfName.value.trim();
    const dir = newShelfResolvedDirectory.value;
    if (!canSubmitAddShelf.value) {
      return;
    }

    addingShelf.value = true;
    addShelfError.value = '';

    try {
      const provider = getBookshelfProvider();
      // The scan interval is not part of creating a shelf; it keeps the backend
      // default and stays adjustable in the modify dialog. Read-only likewise
      // only ever comes from the existing-folder branch.
      const readOnly = newShelfLocationMode.value === 'existing' && newShelfReadOnly.value;
      await provider.addDesktopShelf!(name, dir, '', readOnly);
      await fetchShelves();
      showAddShelfModal.value = false;
      resetAddShelfForm();
    } catch (err) {
      addShelfError.value =
        err instanceof Error ? err.message : t('settings.shelves.addShelfFailed');
    } finally {
      addingShelf.value = false;
    }
  }

  function resetModifyShelfForm(): void {
    modifyShelfName.value = '';
    modifyShelfScanInterval.value = '';
    modifyShelfReadOnly.value = false;
    modifyShelfPath.value = '';
    modifyShelfError.value = '';
  }

  async function requestModifyShelf(shelf: ShelfRef): Promise<void> {
    const provider = getBookshelfProvider();
    if (!provider.getDesktopShelfDetails) {
      return;
    }

    shelfOpError.value = '';
    resetModifyShelfForm();

    try {
      const details = await provider.getDesktopShelfDetails(shelf.id);
      pendingModifyShelf.value = shelf;
      modifyShelfName.value = details.name;
      modifyShelfScanInterval.value = details.scan_interval;
      modifyShelfReadOnly.value = details.read_only;
      modifyShelfPath.value = details.path;
      showModifyShelfModal.value = true;
    } catch (err) {
      shelfOpError.value =
        err instanceof Error ? err.message : t('settings.shelves.modifyShelfFailed');
    }
  }

  function closeModifyShelfModal(): void {
    if (modifyingShelf.value) {
      return;
    }

    showModifyShelfModal.value = false;
    pendingModifyShelf.value = null;
    resetModifyShelfForm();
  }

  async function onSubmitModifyShelf(): Promise<void> {
    const shelf = pendingModifyShelf.value;
    if (!shelf) {
      return;
    }

    const name = modifyShelfName.value.trim();
    const scanInterval = modifyShelfScanInterval.value.trim();
    if (!name) {
      return;
    }

    const provider = getBookshelfProvider();
    if (!provider.modifyDesktopShelf) {
      return;
    }

    modifyingShelf.value = true;
    modifyShelfError.value = '';

    try {
      await provider.modifyDesktopShelf(shelf.id, name, scanInterval, modifyShelfReadOnly.value);
      await fetchShelves();
      showModifyShelfModal.value = false;
      pendingModifyShelf.value = null;
      resetModifyShelfForm();
    } catch (err) {
      modifyShelfError.value =
        err instanceof Error ? err.message : t('settings.shelves.modifyShelfFailed');
    } finally {
      modifyingShelf.value = false;
    }
  }

  return {
    shelves,
    shelvesLoading,
    shelvesError,
    fetchShelves,
    ensureShelvesLoaded,
    shelfOpError,
    removingShelfIDs,
    pendingRemoveShelf,
    shelfRemoveModalError,
    requestRemoveShelf,
    cancelRemoveShelf,
    confirmRemoveShelf,
    showAddShelfModal,
    newShelfName,
    newShelfLocationMode,
    newShelfDirectory,
    newShelfDirectoryError,
    newShelfReadOnly,
    newShelfIDPreview,
    newShelfDefaultPath,
    addingShelf,
    addShelfError,
    canSubmitAddShelf,
    openAddShelfModal,
    closeAddShelfModal,
    onBrowseShelfDirectory,
    onSubmitAddShelf,
    openShelfFolder,
    pendingModifyShelf,
    showModifyShelfModal,
    modifyShelfName,
    modifyShelfScanInterval,
    modifyShelfReadOnly,
    modifyShelfPath,
    modifyingShelf,
    modifyShelfError,
    canSubmitModifyShelf,
    requestModifyShelf,
    closeModifyShelfModal,
    onSubmitModifyShelf
  };
}
