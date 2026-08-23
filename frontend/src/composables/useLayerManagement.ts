import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { CreateLayerParentOption } from '@/components/CreateLayerModal.vue';
import { createLayer, deleteLayer, LayerTransferConflictError, moveLayer, renameLayer } from '@/api/layers';
import { useBookStore } from '@/composables/useBookStore';
import { useLayerStore } from '@/composables/useLayerStore';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { bookshelfWriter, getBookshelfProvider, isWritableProvider } from '@/providers';
import type { BookTransferMode } from '@/api/books';
import {
  booksRouteForLayerPath,
  buildLayerTreeNodes,
  flattenLayerTreePaths,
  getLayerPath,
  normalizeLayerPath
} from '@/utils/layers';
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
    void router.push(booksRouteForLayerPath(layer ?? ''));
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
      await bookshelfWriter().updateBookLayer(currentBook.id, payload.targetLayer);
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

  // Cross-shelf layer transfer: copy or move a whole folder (and everything in
  // it) to another shelf. Like the single-book transfer it reports progress
  // through a task chain and, mirroring that flow, refuses to close mid-transfer
  // rather than pretending a cancel it cannot deliver — the shared composable's
  // stop() only halts client polling; the chain keeps running on the server.
  const transferLayerTarget = ref('');
  const transferLayerMode = ref<BookTransferMode>('copy');

  // The entry is offered only where a folder can actually be transferred: a
  // writable multi-shelf backend (server/desktop) that is not read-only. A
  // reader provider (mobile/pCloud) is not writable, so it never shows.
  const canTransferLayer = computed(() => !readOnly.value && isWritableProvider(getBookshelfProvider()));

  const {
    chain: transferLayerChain,
    status: transferLayerStatus,
    percentage: transferLayerPercentage,
    error: transferLayerError,
    started: transferLayerStarted,
    running: transferLayerRunning,
    finished: transferLayerFinished,
    start: beginTransferLayer,
    reset: resetTransferLayer
  } = useTaskChainProgress({
    onSettled: (status) => {
      if (status === 'failed') {
        return;
      }
      const source = transferLayerTarget.value;
      // A move drops the folder from the source shelf; a copy leaves it. Either
      // way the sidebar's layer tree and per-layer counts have shifted, so pull
      // both stores fresh.
      void Promise.all([fetchLayers(), fetchBooks()]);
      // A move removes the folder the user may be standing in, so send them back
      // to the top when the layer they are viewing has just left the shelf.
      if (
        transferLayerMode.value === 'move' &&
        source &&
        (currentLayer.value === source || currentLayer.value?.startsWith(`${source}/`))
      ) {
        goToLayer(undefined);
      }
    },
    startFailedMessage: () => t('layout.transferLayer.errors.failed'),
    pollFailedMessage: () => t('layout.transferLayer.errors.failed')
  });

  const transferLayerFolderName = computed(() => {
    const segments = transferLayerTarget.value.split('/').filter((segment) => segment.length > 0);
    return segments[segments.length - 1] ?? '';
  });

  function requestTransferLayer(path: string): void {
    if (!canTransferLayer.value) {
      return;
    }
    layerOperationError.value = '';
    transferLayerTarget.value = path;
    resetTransferLayer();
  }

  function cancelTransferLayer(): void {
    // The chain keeps running on the server, so closing mid-transfer would only
    // drop the progress view; refuse it until the transfer settles.
    if (transferLayerRunning.value) {
      return;
    }
    transferLayerTarget.value = '';
    resetTransferLayer();
  }

  // Turns a server refusal into a message the modal can show: the two conflicts
  // become their own readable strings (a book-ID clash lists every colliding ID),
  // and anything else keeps the server's own message.
  function describeTransferLayerError(err: unknown): Error {
    if (err instanceof LayerTransferConflictError) {
      if (err.kind === 'book_id_conflict') {
        return new Error(
          t('layout.transferLayer.errors.conflictBookId', { ids: err.conflictingBookIDs.join(', ') })
        );
      }
      return new Error(t('layout.transferLayer.errors.conflictLayer'));
    }
    return err instanceof Error ? err : new Error(t('layout.transferLayer.errors.failed'));
  }

  async function submitTransferLayer(payload: {
    targetShelfId: string;
    targetParentLayer: string;
    mode: BookTransferMode;
  }): Promise<void> {
    const source = transferLayerTarget.value;
    if (!source || transferLayerStarted.value) {
      return;
    }

    // The folder keeps its own name and nests under the chosen parent, so the full
    // destination path is the parent joined with the folder name (a root parent
    // lands the folder at the target shelf's top level).
    const folderName = transferLayerFolderName.value;
    const parent = payload.targetParentLayer.trim();
    const targetPath = parent ? `${parent}/${folderName}` : folderName;

    transferLayerMode.value = payload.mode;
    // A transfer already in flight for this folder answers with its own chain id,
    // so this attaches to the existing progress instead of scheduling a second.
    await beginTransferLayer(async () => {
      try {
        return await bookshelfWriter().transferLayer(source, payload.targetShelfId, targetPath, payload.mode);
      } catch (err) {
        throw describeTransferLayerError(err);
      }
    });
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
    canTransferLayer,
    transferLayerTarget,
    transferLayerFolderName,
    transferLayerChain,
    transferLayerStatus,
    transferLayerPercentage,
    transferLayerError,
    transferLayerStarted,
    transferLayerRunning,
    transferLayerFinished,
    requestTransferLayer,
    cancelTransferLayer,
    submitTransferLayer,
    requestDeleteLayer,
    cancelPendingDeleteLayer,
    confirmDeleteLayer
  };
}
