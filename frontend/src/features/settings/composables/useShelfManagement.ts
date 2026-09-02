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

  // Read-only is a property of a folder that already exists — a read-only shelf
  // is never created (shelf.NewShelf), which is the whole of the default
  // branch — so leaving the existing-folder branch has to put it back, not just
  // stop showing it.
  watch(newShelfLocationMode, (mode) => {
    if (mode === 'new') {
      newShelfReadOnly.value = false;
    }
  });
  // The shelf id the backend would assign to the typed name, shown live so a
  // name that slugifies to nothing (e.g. a purely non-ASCII "小說") visibly
  // becomes "shelf" before the id is created and frozen as the reading-progress
  // key. Empty when there is no preview: off the desktop, or for an empty name.
  const newShelfIDPreview = ref('');
  // The directory the backend suggests for that id. It is only a default: the
  // form submits it as lib_root when the user never picks a directory, which is
  // what lets a shelf be created from a name alone.
  const newShelfDefaultDirectory = ref('');
  // Latest-wins guard: the async preview lags keystrokes, so a slow earlier
  // response must not overwrite a newer one (or a reset).
  let shelfIDPreviewToken = 0;

  // What the form will actually create, and what the dialog previews — so what
  // it shows is what lands in shelves.json. The branch decides which of the two
  // directories that is; they are never blended, which is what the single path
  // box used to do silently.
  const newShelfEffectiveDirectory = computed(() =>
    newShelfLocationMode.value === 'new'
      ? newShelfDefaultDirectory.value
      : newShelfDirectory.value.trim()
  );

  // A relative path is refused here rather than by Go, which would only answer
  // the finished submit with `shelf directory must be an absolute path`
  // (desktop/shelves.go). Blank while the field is untouched, so an empty form
  // is not an error.
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
    const dir = newShelfEffectiveDirectory.value;
    return dir.length > 0 && isAbsoluteShelfPath(dir);
  });

  async function refreshShelfIDPreview(name: string): Promise<void> {
    const token = ++shelfIDPreviewToken;
    // Drop the previous name's answer before asking for this one. Both fields
    // are derived from the name, so keeping them across the await would show —
    // and, for the directory, submit — the old name's shelf under the new name.
    newShelfIDPreview.value = '';
    newShelfDefaultDirectory.value = '';
    const provider = getBookshelfProvider();
    if (!provider.previewDesktopShelfID || name.trim().length === 0) {
      return;
    }
    try {
      const preview = await provider.previewDesktopShelfID(name.trim());
      if (token === shelfIDPreviewToken) {
        newShelfIDPreview.value = preview.id;
        newShelfDefaultDirectory.value = preview.defaultPath;
      }
    } catch {
      // The fields were already cleared above; a failed preview leaves them so.
    }
  }

  watch(newShelfName, (name) => {
    void refreshShelfIDPreview(name);
  });

  const pendingModifyShelf = ref<ShelfRef | null>(null);
  const showModifyShelfModal = ref(false);
  const modifyShelfName = ref('');
  const modifyShelfScanInterval = ref('');
  const modifyShelfBookCheckInterval = ref('');
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
    // the field after the form is cleared.
    shelfIDPreviewToken++;
    newShelfIDPreview.value = '';
    newShelfDefaultDirectory.value = '';
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
    const dir = newShelfEffectiveDirectory.value;
    if (!canSubmitAddShelf.value) {
      return;
    }

    addingShelf.value = true;
    addShelfError.value = '';

    try {
      const provider = getBookshelfProvider();
      // Neither interval is part of creating a shelf: both keep the backend
      // default and stay adjustable in the modify dialog. Read-only likewise
      // only ever comes from the existing-folder branch.
      await provider.addDesktopShelf!({
        name,
        libRoot: dir,
        scanInterval: '',
        bookCheckInterval: '',
        readOnly: newShelfLocationMode.value === 'existing' && newShelfReadOnly.value
      });
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
    modifyShelfBookCheckInterval.value = '';
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
      modifyShelfBookCheckInterval.value = details.book_check_interval;
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
    const bookCheckInterval = modifyShelfBookCheckInterval.value.trim();
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
      await provider.modifyDesktopShelf({
        shelfID: shelf.id,
        name,
        scanInterval,
        bookCheckInterval,
        readOnly: modifyShelfReadOnly.value
      });
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
    newShelfEffectiveDirectory,
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
    modifyShelfBookCheckInterval,
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
