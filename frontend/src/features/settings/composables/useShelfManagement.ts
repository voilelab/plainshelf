import { computed, ref, watch } from 'vue';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { getBookshelfProvider } from '@/providers';
import { useI18n } from '@/i18n';

interface ShelfRef {
  id: string;
  name: string;
}

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
  const newShelfDirectory = ref('');
  const newShelfScanInterval = ref('');
  const newShelfBookCheckInterval = ref('');
  const newShelfReadOnly = ref(false);
  const addingShelf = ref(false);
  const addShelfError = ref('');
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

  // What the form will actually create. A directory the user typed or browsed
  // to wins over the suggestion, and it is this value the panel previews — so
  // what the dialog shows is what lands in shelves.json.
  const newShelfEffectiveDirectory = computed(() => {
    const chosen = newShelfDirectory.value.trim();
    if (chosen) {
      return chosen;
    }
    // A read-only shelf is never created, so its lib_root must already exist
    // (shelf.NewShelf). The default names a directory that by construction does
    // not, so offering it here would advertise a name-only flow that always
    // fails: read-only creation needs a directory the user points at.
    return newShelfReadOnly.value ? '' : newShelfDefaultDirectory.value;
  });
  const canSubmitAddShelf = computed(
    () => newShelfName.value.trim().length > 0 && newShelfEffectiveDirectory.value.length > 0
  );

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
    newShelfDirectory.value = '';
    newShelfScanInterval.value = '';
    newShelfBookCheckInterval.value = '';
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
    const scanInterval = newShelfScanInterval.value.trim();
    const bookCheckInterval = newShelfBookCheckInterval.value.trim();
    if (!name || !dir) {
      return;
    }

    addingShelf.value = true;
    addShelfError.value = '';

    try {
      const provider = getBookshelfProvider();
      await provider.addDesktopShelf!({
        name,
        libRoot: dir,
        scanInterval,
        bookCheckInterval,
        readOnly: newShelfReadOnly.value
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
    newShelfDirectory,
    newShelfScanInterval,
    newShelfBookCheckInterval,
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
