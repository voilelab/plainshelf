import type { DesktopQRShelf, DesktopQRResult } from '@/api/desktop';
import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import type { DesktopShelfDetails } from '@/api/desktop';
import { computed, ref, watch } from 'vue';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useI18n } from '@/i18n';
import { isAbsoluteShelfPath } from '@/features/settings/utils/shelfPath';
import {
  shelfSourceCheckMessageKey,
  shelfSourceNeedsSecret,
  type ShelfSourceKind
} from '@/utils/shelfSourceCheck';

interface ShelfRef {
  id: string;
  name: string;
}

type ShelfLocationMode = 'new' | 'existing' | 'remote' | 'qr';

export function useShelfManagement() {
  const { t } = useI18n();
  const store = useShelvesStore();
  const managementDetails = ref<DesktopShelfDetails[] | null>(null);
  const shelvesLoading = ref(false);
  const shelvesError = ref('');
  const shelves = computed(() =>
    managementDetails.value
      ? managementDetails.value.map((detail) => ({
          ...detail,
          readOnly: detail.read_only
        }))
      : store.shelves.value.map((shelf) => ({
          ...shelf,
          source: '',
          url: '',
          open_error: ''
        }))
  );

  async function loadManagementDetails(): Promise<void> {
    const list = getPlatformCapabilities().shelves?.listShelfDetails;
    if (list) managementDetails.value = await list();
  }

  async function refreshShelves(ensure: boolean): Promise<void> {
    shelvesLoading.value = true;
    shelvesError.value = '';
    try {
      await (ensure ? store.ensureShelvesLoaded() : store.fetchShelves());
      await loadManagementDetails();
      shelvesError.value = store.error.value;
    } catch (err) {
      shelvesError.value = err instanceof Error ? err.message : t('settings.shelves.loadFailed');
    } finally {
      shelvesLoading.value = false;
    }
  }

  const fetchShelves = () => refreshShelves(false);
  const ensureShelvesLoaded = () => refreshShelves(true);
  const retryingShelfIDs = ref(new Set<string>());

  async function retryShelf(id: string): Promise<void> {
    const retry = getPlatformCapabilities().shelves?.retryShelf;
    if (!retry || retryingShelfIDs.value.has(id)) return;
    retryingShelfIDs.value = new Set([...retryingShelfIDs.value, id]);
    shelfOpError.value = '';
    try {
      await retry(id);
      await fetchShelves();
    } catch {
      shelfOpError.value = t('settings.shelves.virtualOpenFailed');
      await fetchShelves();
    } finally {
      const next = new Set(retryingShelfIDs.value);
      next.delete(id);
      retryingShelfIDs.value = next;
    }
  }

  const shelfOpError = ref('');

  const removingShelfIDs = ref<Set<string>>(new Set());
  const pendingRemoveShelf = ref<ShelfRef | null>(null);
  const shelfRemoveModalError = ref('');

  const showAddShelfModal = ref(false);
  const newShelfName = ref('');
  const newShelfLocationMode = ref<ShelfLocationMode>('new');
  const newShelfDirectory = ref('');
  const newShelfReadOnly = ref(false);
  const newShelfSourceKind = ref<ShelfSourceKind>('plainshelf');
  const newShelfSourceURL = ref('');
  const newShelfSourceUsername = ref('');
  const newShelfSourceSecret = ref('');
  const addingShelf = ref(false);
  const qrText = ref('');
  const qrShelves = ref<DesktopQRShelf[]>([]);
  const qrResults = ref<DesktopQRResult[]>([]);
  let qrPreviewToken = 0;
  watch(qrText, () => {
    qrPreviewToken++;
    qrShelves.value = [];
    qrResults.value = [];
  }, { flush: 'sync' });

  async function previewQR(): Promise<void> {
    const text = qrText.value;
    const token = ++qrPreviewToken;
    addShelfError.value = '';
    try {
      const shelves = await getPlatformCapabilities().shelves!.previewLANShareQR!(text);
      if (token === qrPreviewToken) qrShelves.value = shelves;
    } catch (err) {
      if (token === qrPreviewToken) addShelfError.value = err instanceof Error ? err.message : t('settings.shelves.qrFailed');
    }
  }

  function qrStatus(status: string): string {
    if (status === 'added') return t('settings.shelves.qrAdded');
    if (status === 'exists') return t('settings.shelves.qrExists');
    if (status === 'failed') return t('settings.shelves.qrFailed');
    const key = shelfSourceCheckMessageKey({ code: status, books: 0 });
    return key ? t(key) : t('settings.shelves.qrFailed');
  }
  const sourceUsername = () =>
    newShelfSourceKind.value === 'webdav' ? newShelfSourceUsername.value.trim() : '';
  const addShelfError = ref('');

  watch(newShelfLocationMode, (mode) => {
    if (mode !== 'existing') {
      newShelfReadOnly.value = false;
    }
  });
  const newShelfIDPreview = ref('');
  const newShelfDefaultDirectory = ref('');
  let shelfIDPreviewToken = 0;

  const newShelfEffectiveDirectory = computed(() =>
    newShelfLocationMode.value === 'new'
      ? newShelfDefaultDirectory.value
      : newShelfDirectory.value.trim()
  );

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
    if (newShelfLocationMode.value === 'qr') return qrShelves.value.length > 0;
    if (newShelfName.value.trim().length === 0) {
      return false;
    }
    if (newShelfLocationMode.value === 'remote') {
      return (
        newShelfSourceURL.value.trim().length > 0 &&
        (!shelfSourceNeedsSecret(newShelfSourceKind.value, sourceUsername()) ||
          newShelfSourceSecret.value.trim().length > 0)
      );
    }
    const dir = newShelfEffectiveDirectory.value;
    return dir.length > 0 && isAbsoluteShelfPath(dir);
  });

  async function refreshShelfIDPreview(name: string): Promise<void> {
    const token = ++shelfIDPreviewToken;
    newShelfIDPreview.value = '';
    newShelfDefaultDirectory.value = '';
    const platform = getPlatformCapabilities();
    if (!platform.shelves?.previewShelfID || name.trim().length === 0) {
      return;
    }
    try {
      const preview = await platform.shelves!.previewShelfID(name.trim());
      if (token === shelfIDPreviewToken) {
        newShelfIDPreview.value = preview.id;
        newShelfDefaultDirectory.value = preview.defaultPath;
      }
    } catch {
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
  const modifyShelfIsVirtual = ref(false);
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
    const platform = getPlatformCapabilities();
    if (!platform.shelves?.removeShelf) {
      return;
    }

    removingShelfIDs.value = new Set([...removingShelfIDs.value, shelf.id]);

    shelfOpError.value = '';
    try {
      await platform.shelves!.removeShelf(shelf.id);
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
    qrText.value = '';
    qrShelves.value = [];
    qrResults.value = [];
    newShelfName.value = '';
    newShelfLocationMode.value = 'new';
    newShelfDirectory.value = '';
    newShelfReadOnly.value = false;
    newShelfSourceKind.value = 'plainshelf';
    newShelfSourceURL.value = '';
    newShelfSourceUsername.value = '';
    newShelfSourceSecret.value = '';
    addShelfError.value = '';
    shelfIDPreviewToken++;
    newShelfIDPreview.value = '';
    newShelfDefaultDirectory.value = '';
  }

  async function openShelfFolder(shelfID: string): Promise<void> {
    const platform = getPlatformCapabilities();
    if (!platform.openShelfFolder) {
      return;
    }
    shelfOpError.value = '';
    try {
      await platform.openShelfFolder(shelfID);
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
    const platform = getPlatformCapabilities();
    if (!platform.chooseShelfDirectory) {
      return;
    }
    const dir = await platform.chooseShelfDirectory();
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
      const platform = getPlatformCapabilities();
      if (newShelfLocationMode.value === 'qr') {
        const results = await platform.shelves!.importLANShareQR!(qrText.value);
        qrText.value = '';
        qrShelves.value = [];
        qrResults.value = results;
        await fetchShelves();
        return;
      }
      if (newShelfLocationMode.value === 'remote') {
        await addRemoteShelf(name);
        return;
      }
      await platform.shelves!.addShelf!({
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

  async function addRemoteShelf(name: string): Promise<void> {
    const platform = getPlatformCapabilities();
    const remote = {
      source: newShelfSourceKind.value,
      url: newShelfSourceURL.value.trim(),
      username: sourceUsername(),
      secret: newShelfSourceSecret.value.trim()
    };
    const check = await platform.shelves!.checkShelfSource!(remote);
    const messageKey = shelfSourceCheckMessageKey(check);
    if (messageKey) {
      addShelfError.value = t(messageKey);
      return;
    }
    await platform.shelves!.addShelf!({
      name,
      libRoot: '',
      scanInterval: '',
      bookCheckInterval: '',
      readOnly: true,
      remote
    });
    await fetchShelves();
    showAddShelfModal.value = false;
    resetAddShelfForm();
  }

  function resetModifyShelfForm(): void {
    modifyShelfName.value = '';
    modifyShelfScanInterval.value = '';
    modifyShelfBookCheckInterval.value = '';
    modifyShelfReadOnly.value = false;
    modifyShelfPath.value = '';
    modifyShelfIsVirtual.value = false;
    modifyShelfError.value = '';
  }

  async function requestModifyShelf(shelf: ShelfRef): Promise<void> {
    const platform = getPlatformCapabilities();
    if (!platform.shelves?.getShelfDetails) {
      return;
    }

    shelfOpError.value = '';
    resetModifyShelfForm();

    try {
      const details = await platform.shelves!.getShelfDetails(shelf.id);
      pendingModifyShelf.value = shelf;
      modifyShelfName.value = details.name;
      modifyShelfScanInterval.value = details.scan_interval;
      modifyShelfBookCheckInterval.value = details.book_check_interval;
      modifyShelfReadOnly.value = details.read_only;
      modifyShelfIsVirtual.value = Boolean(details.source);
      modifyShelfPath.value = details.source ? details.url : details.path;
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

    const platform = getPlatformCapabilities();
    if (!platform.shelves?.modifyShelf) {
      return;
    }

    modifyingShelf.value = true;
    modifyShelfError.value = '';

    try {
      await platform.shelves!.modifyShelf({
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
    retryShelf,
    retryingShelfIDs,
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
    qrText,
    qrShelves,
    qrResults,
    previewQR,
    qrStatus,
    showAddShelfModal,
    newShelfName,
    newShelfLocationMode,
    newShelfDirectory,
    newShelfDirectoryError,
    newShelfReadOnly,
    newShelfSourceKind,
    newShelfSourceURL,
    newShelfSourceUsername,
    newShelfSourceSecret,
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
    modifyShelfIsVirtual,
    modifyingShelf,
    modifyShelfError,
    canSubmitModifyShelf,
    requestModifyShelf,
    closeModifyShelfModal,
    onSubmitModifyShelf
  };
}
