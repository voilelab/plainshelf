import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { CreateLayerParentOption } from '@/components/CreateLayerModal.vue';
import { createLayer, deleteLayer, moveLayer, renameLayer } from '@/api/layers';
import { useBookStore } from '@/composables/useBookStore';
import { useLayerStore } from '@/composables/useLayerStore';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { getBookshelfProvider } from '@/providers';
import { buildLayerTreeNodes, flattenLayerTreePaths, getLayerPath, normalizeLayerPath } from '@/utils/layers';
import { useI18n } from '@/i18n';
import { useBookBatchOperations } from '@/composables/useBookBatchOperations';
import type { Book } from '@/types/book';

/**
 * Where to navigate after `path` is renamed to `nextName`, given the layer the
 * user is currently viewing. Returns undefined when the current layer is not the
 * renamed one or one of its descendants, meaning no navigation is needed.
 */
export function renamedLayerDestination(
  current: string | undefined,
  path: string,
  nextName: string
): string | undefined {
  if (current === undefined || (current !== path && !current.startsWith(`${path}/`))) {
    return undefined;
  }

  const parent = path.split('/').filter((segment) => segment.length > 0).slice(0, -1);
  const renamedPath = [...parent, nextName].join('/');
  return current === path ? renamedPath : `${renamedPath}${current.slice(path.length)}`;
}

/**
 * Where to navigate after `layerPath` is moved under `targetLayer`, given the
 * layer the user is currently viewing. Returns undefined when no navigation is
 * needed, including for a root path that has no name segment to move.
 */
export function movedLayerDestination(
  current: string | undefined,
  layerPath: string,
  targetLayer: string
): string | undefined {
  if (current === undefined || (current !== layerPath && !current.startsWith(`${layerPath}/`))) {
    return undefined;
  }

  const layerSegments = layerPath.split('/').filter((segment) => segment.length > 0);
  const layerName = layerSegments[layerSegments.length - 1];
  if (!layerName) {
    return undefined;
  }

  const targetSegments = targetLayer === '/' ? [] : targetLayer.split('/').filter(Boolean);
  const movedPath = [...targetSegments, layerName].join('/');
  return current === layerPath ? movedPath : `${movedPath}${current.slice(layerPath.length)}`;
}

/**
 * The sidebar's layer tree operations: create, rename, move, delete, open the
 * folder on desktop, and move a book between layers. Owns the busy/error state
 * each operation reports, and keeps the URL pointing somewhere real when the
 * layer being viewed is renamed, moved or deleted.
 */
export function useLayerManagement() {
  const route = useRoute();
  const router = useRouter();
  const { t } = useI18n();
  const { books, fetchBooks } = useBookStore();
  const { layers, fetchLayers } = useLayerStore();
  const { writesEnabled } = useWriteAccess();
  const readOnly = computed(() => !writesEnabled.value);
  const batchOperations = useBookBatchOperations();

  const moveBookError = ref('');
  const showCreateLayerModal = ref(false);
  const creatingLayer = ref(false);
  const createLayerError = ref('');
  const deleteLayerError = ref('');
  const layerOperationError = ref('');
  const pendingRenameLayerPath = ref('');
  const renameLayerError = ref('');
  const renamingLayer = ref(false);
  const deletingLayerMap = ref<Record<string, boolean>>({});
  const pendingDeleteLayerPath = ref('');

  const currentLayer = computed(() => {
    const q = route.query.layers;
    return typeof q === 'string' && q.length > 0 ? q : undefined;
  });

  const layerTree = computed(() => buildLayerTreeNodes(layers.value));
  const canOpenLayerFolder = computed(() => Boolean(getBookshelfProvider().openDesktopLayerFolder));
  const createLayerParentOptions = computed<CreateLayerParentOption[]>(() => [
    { value: '/', label: t('layout.createLayer.rootOption'), depth: 0 },
    ...flattenLayerTreePaths(layerTree.value).map((option) => ({
      value: option.path,
      label: option.path,
      depth: option.depth + 1
    }))
  ]);
  const createLayerDefaultParent = computed(() => normalizeLayerPath(currentLayer.value ?? '') || '/');
  const isDeletingPendingLayer = computed(
    () => pendingDeleteLayerPath.value.length > 0 && (deletingLayerMap.value[pendingDeleteLayerPath.value] ?? false)
  );
  const pendingRenameLayerName = computed(() => {
    const segments = pendingRenameLayerPath.value.split('/').filter((segment) => segment.length > 0);
    return segments[segments.length - 1] ?? '';
  });
  const isRenamingPendingLayer = computed(() => pendingRenameLayerPath.value.length > 0 && renamingLayer.value);

  function goToLayer(layer: string | undefined): void {
    const query: Record<string, string> = { page: '1' };
    if (layer) query.layers = layer;
    void router.push({ path: '/books', query });
  }

  /** Clears the errors a shelf switch invalidates. */
  function clearLayerErrors(): void {
    deleteLayerError.value = '';
    layerOperationError.value = '';
    moveBookError.value = '';
    createLayerError.value = '';
  }

  function normalizeLayerSelectionPath(path: string): string | undefined {
    const trimmed = path.trim();
    if (trimmed === '') {
      return undefined;
    }
    if (trimmed === '/') {
      return '/';
    }

    const normalized = normalizeLayerPath(trimmed);
    return normalized.length > 0 ? normalized : undefined;
  }

  function onSelectLayer(path: string): void {
    deleteLayerError.value = '';
    layerOperationError.value = '';
    goToLayer(normalizeLayerSelectionPath(path));
  }

  function openCreateLayerModal(): void {
    if (readOnly.value) {
      return;
    }

    createLayerError.value = '';
    showCreateLayerModal.value = true;
  }

  function closeCreateLayerModal(): void {
    if (creatingLayer.value) {
      return;
    }

    showCreateLayerModal.value = false;
    createLayerError.value = '';
  }

  async function onSubmitCreateLayer(payload: { parentPath: string; name: string }): Promise<void> {
    if (readOnly.value) {
      createLayerError.value = t('layout.readOnly.writeDisabled');
      return;
    }

    const name = payload.name.trim();
    if (!name || name.includes('/')) {
      createLayerError.value = t('layout.createLayer.invalidName');
      return;
    }

    // normalizeLayerPath drops empty segments, so a '/' parent joins cleanly.
    const normalized = normalizeLayerPath(`${payload.parentPath}/${name}`);
    if (!normalized) {
      createLayerError.value = t('layout.layerErrors.emptyPath');
      return;
    }

    creatingLayer.value = true;
    createLayerError.value = '';

    try {
      await createLayer(normalized);
      await fetchLayers();

      showCreateLayerModal.value = false;
      goToLayer(normalized);
    } catch (err) {
      const message = err instanceof Error ? err.message : t('layout.layerErrors.createFailed');

      if (message === 'Layer path cannot be empty') {
        createLayerError.value = t('layout.layerErrors.emptyPath');
      } else if (message === 'Failed to create layer') {
        createLayerError.value = t('layout.layerErrors.createFailed');
      } else {
        createLayerError.value = message || t('layout.layerErrors.createFailed');
      }
    } finally {
      creatingLayer.value = false;
    }
  }

  async function onMoveBook(payload: { bookIds: string[]; targetLayer: string; batch: boolean }): Promise<void> {
    if (readOnly.value) {
      moveBookError.value = t('layout.readOnly.writeDisabled');
      return;
    }
    moveBookError.value = '';
    layerOperationError.value = '';

    const selectedBooks = payload.bookIds
      .map((id) => books.value.find((item) => item.id === id))
      .filter((book): book is Book => Boolean(book));
    if (selectedBooks.length === 0) {
      moveBookError.value = t('layout.moveBookErrors.notFound');
      return;
    }

    if (payload.batch) {
      const target = payload.targetLayer === '/' ? [] : normalizeLayerPath(payload.targetLayer).split('/').filter(Boolean);
      const titles = Object.fromEntries(selectedBooks.map((book) => [book.id, book.title]));
      await batchOperations.startMove(payload.bookIds, target, titles);
      return;
    }

    const currentBook = selectedBooks[0];
    const currentLayerPath = getLayerPath(currentBook);
    if (currentLayerPath === payload.targetLayer) {
      return;
    }

    try {
      await getBookshelfProvider().updateBookLayer(currentBook.id, payload.targetLayer);
      await fetchBooks();
    } catch (err) {
      moveBookError.value = err instanceof Error ? err.message : t('layout.moveBookErrors.failed');
    }
  }

  function requestRenameLayer(path: string): void {
    if (readOnly.value) {
      layerOperationError.value = t('layout.readOnly.writeDisabled');
      return;
    }

    pendingRenameLayerPath.value = path;
    renameLayerError.value = '';
    layerOperationError.value = '';
  }

  function cancelPendingRenameLayer(): void {
    if (renamingLayer.value) {
      return;
    }

    pendingRenameLayerPath.value = '';
    renameLayerError.value = '';
  }

  async function confirmRenameLayer(nextName: string): Promise<void> {
    const path = pendingRenameLayerPath.value;
    if (!path || renamingLayer.value) {
      return;
    }

    if (!nextName || nextName === pendingRenameLayerName.value) {
      renameLayerError.value = t('layout.renameLayer.invalid');
      return;
    }

    renamingLayer.value = true;
    renameLayerError.value = '';
    layerOperationError.value = '';

    try {
      await renameLayer(path, nextName);
      await Promise.all([fetchLayers(), fetchBooks()]);

      const destination = renamedLayerDestination(currentLayer.value, path, nextName);
      if (destination !== undefined) {
        goToLayer(destination);
      }

      pendingRenameLayerPath.value = '';
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      if (message === 'Invalid layer name') {
        renameLayerError.value = t('layout.renameLayer.invalid');
      } else {
        renameLayerError.value = message || t('layout.renameLayer.failed');
      }
    } finally {
      renamingLayer.value = false;
    }
  }

  async function onMoveLayer(payload: { layerPath: string; targetLayer: string }): Promise<void> {
    if (readOnly.value) {
      layerOperationError.value = t('layout.readOnly.writeDisabled');
      return;
    }
    layerOperationError.value = '';

    try {
      await moveLayer(payload.layerPath, payload.targetLayer);
      await Promise.all([fetchLayers(), fetchBooks()]);

      const destination = movedLayerDestination(currentLayer.value, payload.layerPath, payload.targetLayer);
      if (destination !== undefined) {
        goToLayer(destination);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      layerOperationError.value = message || t('layout.moveLayer.failed');
    }
  }

  async function onOpenLayerFolder(path: string): Promise<void> {
    layerOperationError.value = '';
    const openDesktopLayerFolder = getBookshelfProvider().openDesktopLayerFolder;
    if (!openDesktopLayerFolder) {
      return;
    }

    try {
      await openDesktopLayerFolder(path);
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      layerOperationError.value = message || t('layout.openLayerFolder.failed');
    }
  }

  function requestDeleteLayer(path: string): void {
    if (readOnly.value) {
      deleteLayerError.value = t('layout.readOnly.writeDisabled');
      return;
    }
    if (deletingLayerMap.value[path]) {
      return;
    }

    deleteLayerError.value = '';
    pendingDeleteLayerPath.value = path;
  }

  function cancelPendingDeleteLayer(): void {
    if (isDeletingPendingLayer.value) {
      return;
    }

    pendingDeleteLayerPath.value = '';
    deleteLayerError.value = '';
  }

  async function confirmDeleteLayer(): Promise<void> {
    const path = pendingDeleteLayerPath.value;
    if (!path || deletingLayerMap.value[path]) {
      return;
    }

    deleteLayerError.value = '';
    deletingLayerMap.value = {
      ...deletingLayerMap.value,
      [path]: true
    };

    try {
      await deleteLayer(path);
      await Promise.all([fetchLayers(), fetchBooks()]);

      if (currentLayer.value === path) {
        goToLayer(undefined);
      }

      pendingDeleteLayerPath.value = '';
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      if (message === 'Cannot delete this layer because it is not empty.') {
        deleteLayerError.value = t('layout.deleteLayer.notEmpty');
      } else if (message) {
        deleteLayerError.value = message;
      } else {
        deleteLayerError.value = t('layout.deleteLayer.failed');
      }
    } finally {
      const { [path]: _deleted, ...rest } = deletingLayerMap.value;
      deletingLayerMap.value = rest;
    }
  }

  return {
    moveBookError,
    showCreateLayerModal,
    creatingLayer,
    createLayerError,
    deleteLayerError,
    layerOperationError,
    pendingRenameLayerPath,
    renameLayerError,
    deletingLayerMap,
    pendingDeleteLayerPath,
    currentLayer,
    layerTree,
    canOpenLayerFolder,
    createLayerParentOptions,
    createLayerDefaultParent,
    isDeletingPendingLayer,
    pendingRenameLayerName,
    isRenamingPendingLayer,
    clearLayerErrors,
    onSelectLayer,
    openCreateLayerModal,
    closeCreateLayerModal,
    onSubmitCreateLayer,
    onMoveBook,
    requestRenameLayer,
    cancelPendingRenameLayer,
    confirmRenameLayer,
    onMoveLayer,
    onOpenLayerFolder,
    requestDeleteLayer,
    cancelPendingDeleteLayer,
    confirmDeleteLayer
  };
}
