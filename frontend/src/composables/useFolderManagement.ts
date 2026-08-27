import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { createFolder, deleteFolder, FolderTransferConflictError, moveFolder, renameFolder } from '@/api/folders';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { bookshelfWriter, getBookshelfProvider, isWritableProvider } from '@/providers';
import type { BookTransferMode } from '@/api/books';
import {
  booksRouteForFolderPath,
  buildFolderTreeNodes,
  getFolderPath,
  normalizeFolderPath
} from '@/utils/folders';
import { useI18n } from '@/i18n';
import { useBookBatchOperations } from '@/composables/useBookBatchOperations';
import type { Book } from '@/types/book';

/**
 * Where to navigate after `path` is renamed to `nextName`, given the folder the
 * user is currently viewing. Returns undefined when the current folder is not the
 * renamed one or one of its descendants, meaning no navigation is needed.
 */
export function renamedFolderDestination(
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
 * Where to navigate after `folderPath` is moved under `targetFolder`, given the
 * folder the user is currently viewing. Returns undefined when no navigation is
 * needed, including for a root path that has no name segment to move.
 */
export function movedFolderDestination(
  current: string | undefined,
  folderPath: string,
  targetFolder: string
): string | undefined {
  if (current === undefined || (current !== folderPath && !current.startsWith(`${folderPath}/`))) {
    return undefined;
  }

  const folderSegments = folderPath.split('/').filter((segment) => segment.length > 0);
  const folderName = folderSegments[folderSegments.length - 1];
  if (!folderName) {
    return undefined;
  }

  const targetSegments = targetFolder === '/' ? [] : targetFolder.split('/').filter(Boolean);
  const movedPath = [...targetSegments, folderName].join('/');
  return current === folderPath ? movedPath : `${movedPath}${current.slice(folderPath.length)}`;
}

/** Resolves a context-menu parent and a folder name to the path sent to the API. */
export function createdFolderDestination(parentPath: string, name: string): string {
  return normalizeFolderPath(`${parentPath}/${name}`);
}

/**
 * The sidebar's folder tree operations: create, rename, move, delete, open the
 * folder on desktop, and move a book between folders. Owns the busy/error state
 * each operation reports, and keeps the URL pointing somewhere real when the
 * folder being viewed is renamed, moved or deleted.
 */
export function useFolderManagement() {
  const route = useRoute();
  const router = useRouter();
  const { t } = useI18n();
  const { books, fetchBooks } = useBookStore();
  const { folders, fetchFolders } = useFolderStore();
  const { writesEnabled } = useWriteAccess();
  const readOnly = computed(() => !writesEnabled.value);
  const batchOperations = useBookBatchOperations();

  const moveBookError = ref('');
  const showCreateFolderModal = ref(false);
  const pendingCreateFolderParentPath = ref('/');
  const creatingFolder = ref(false);
  const createFolderError = ref('');
  const deleteFolderError = ref('');
  const folderOperationError = ref('');
  const pendingRenameFolderPath = ref('');
  const renameFolderError = ref('');
  const renamingFolder = ref(false);
  const deletingFolderMap = ref<Record<string, boolean>>({});
  const pendingDeleteFolderPath = ref('');

  const currentFolder = computed(() => {
    const q = route.query.folders;
    return typeof q === 'string' && q.length > 0 ? q : undefined;
  });

  const folderTree = computed(() => buildFolderTreeNodes(folders.value));
  const canOpenFolder = computed(() => Boolean(getBookshelfProvider().openDesktopFolder));
  const isDeletingPendingFolder = computed(
    () => pendingDeleteFolderPath.value.length > 0 && (deletingFolderMap.value[pendingDeleteFolderPath.value] ?? false)
  );
  const pendingRenameFolderName = computed(() => {
    const segments = pendingRenameFolderPath.value.split('/').filter((segment) => segment.length > 0);
    return segments[segments.length - 1] ?? '';
  });
  const isRenamingPendingFolder = computed(() => pendingRenameFolderPath.value.length > 0 && renamingFolder.value);

  function goToFolder(folder: string | undefined): void {
    void router.push(booksRouteForFolderPath(folder ?? ''));
  }

  /** Clears the errors a shelf switch invalidates. */
  function clearFolderErrors(): void {
    deleteFolderError.value = '';
    folderOperationError.value = '';
    moveBookError.value = '';
    createFolderError.value = '';
  }

  function normalizeFolderSelectionPath(path: string): string | undefined {
    const trimmed = path.trim();
    if (trimmed === '') {
      return undefined;
    }
    if (trimmed === '/') {
      return '/';
    }

    const normalized = normalizeFolderPath(trimmed);
    return normalized.length > 0 ? normalized : undefined;
  }

  function onSelectFolder(path: string): void {
    deleteFolderError.value = '';
    folderOperationError.value = '';
    goToFolder(normalizeFolderSelectionPath(path));
  }

  function openCreateFolderModal(parentPath: string): void {
    if (readOnly.value) {
      return;
    }

    pendingCreateFolderParentPath.value = normalizeFolderPath(parentPath) || '/';
    createFolderError.value = '';
    showCreateFolderModal.value = true;
  }

  function closeCreateFolderModal(): void {
    if (creatingFolder.value) {
      return;
    }

    showCreateFolderModal.value = false;
    createFolderError.value = '';
  }

  async function onSubmitCreateFolder(payload: { name: string }): Promise<void> {
    if (readOnly.value) {
      createFolderError.value = t('layout.readOnly.writeDisabled');
      return;
    }

    const name = payload.name.trim();
    if (!name || name.includes('/')) {
      createFolderError.value = t('layout.createFolder.invalidName');
      return;
    }

    const normalized = createdFolderDestination(pendingCreateFolderParentPath.value, name);
    if (!normalized) {
      createFolderError.value = t('layout.folderErrors.emptyPath');
      return;
    }

    creatingFolder.value = true;
    createFolderError.value = '';

    try {
      await createFolder(normalized);
      await fetchFolders();

      showCreateFolderModal.value = false;
      goToFolder(normalized);
    } catch (err) {
      const message = err instanceof Error ? err.message : t('layout.folderErrors.createFailed');

      if (message === 'Folder path cannot be empty') {
        createFolderError.value = t('layout.folderErrors.emptyPath');
      } else if (message === 'Failed to create folder') {
        createFolderError.value = t('layout.folderErrors.createFailed');
      } else {
        createFolderError.value = message || t('layout.folderErrors.createFailed');
      }
    } finally {
      creatingFolder.value = false;
    }
  }

  async function onMoveBook(payload: { bookIds: string[]; targetFolder: string; batch: boolean }): Promise<void> {
    if (readOnly.value) {
      moveBookError.value = t('layout.readOnly.writeDisabled');
      return;
    }
    moveBookError.value = '';
    folderOperationError.value = '';

    const selectedBooks = payload.bookIds
      .map((id) => books.value.find((item) => item.id === id))
      .filter((book): book is Book => Boolean(book));
    if (selectedBooks.length === 0) {
      moveBookError.value = t('layout.moveBookErrors.notFound');
      return;
    }

    if (payload.batch) {
      const target = payload.targetFolder === '/' ? [] : normalizeFolderPath(payload.targetFolder).split('/').filter(Boolean);
      const titles = Object.fromEntries(selectedBooks.map((book) => [book.id, book.title]));
      await batchOperations.startMove(payload.bookIds, target, titles);
      return;
    }

    const currentBook = selectedBooks[0];
    const currentFolderPath = getFolderPath(currentBook);
    if (currentFolderPath === payload.targetFolder) {
      return;
    }

    try {
      await bookshelfWriter().updateBookFolder(currentBook.id, payload.targetFolder);
      await fetchBooks();
    } catch (err) {
      moveBookError.value = err instanceof Error ? err.message : t('layout.moveBookErrors.failed');
    }
  }

  function requestRenameFolder(path: string): void {
    if (readOnly.value) {
      folderOperationError.value = t('layout.readOnly.writeDisabled');
      return;
    }

    pendingRenameFolderPath.value = path;
    renameFolderError.value = '';
    folderOperationError.value = '';
  }

  function cancelPendingRenameFolder(): void {
    if (renamingFolder.value) {
      return;
    }

    pendingRenameFolderPath.value = '';
    renameFolderError.value = '';
  }

  async function confirmRenameFolder(nextName: string): Promise<void> {
    const path = pendingRenameFolderPath.value;
    if (!path || renamingFolder.value) {
      return;
    }

    if (!nextName || nextName === pendingRenameFolderName.value) {
      renameFolderError.value = t('layout.renameFolder.invalid');
      return;
    }

    renamingFolder.value = true;
    renameFolderError.value = '';
    folderOperationError.value = '';

    try {
      await renameFolder(path, nextName);
      await Promise.all([fetchFolders(), fetchBooks()]);

      const destination = renamedFolderDestination(currentFolder.value, path, nextName);
      if (destination !== undefined) {
        goToFolder(destination);
      }

      pendingRenameFolderPath.value = '';
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      if (message === 'Invalid folder name') {
        renameFolderError.value = t('layout.renameFolder.invalid');
      } else {
        renameFolderError.value = message || t('layout.renameFolder.failed');
      }
    } finally {
      renamingFolder.value = false;
    }
  }

  async function onMoveFolder(payload: { folderPath: string; targetFolder: string }): Promise<void> {
    if (readOnly.value) {
      folderOperationError.value = t('layout.readOnly.writeDisabled');
      return;
    }
    folderOperationError.value = '';

    try {
      await moveFolder(payload.folderPath, payload.targetFolder);
      await Promise.all([fetchFolders(), fetchBooks()]);

      const destination = movedFolderDestination(currentFolder.value, payload.folderPath, payload.targetFolder);
      if (destination !== undefined) {
        goToFolder(destination);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      folderOperationError.value = message || t('layout.moveFolder.failed');
    }
  }

  async function onOpenFolder(path: string): Promise<void> {
    folderOperationError.value = '';
    const openDesktopFolder = getBookshelfProvider().openDesktopFolder;
    if (!openDesktopFolder) {
      return;
    }

    try {
      await openDesktopFolder(path);
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      folderOperationError.value = message || t('layout.openFolder.failed');
    }
  }

  // Cross-shelf folder transfer: copy or move a whole folder (and everything in
  // it) to another shelf. Like the single-book transfer it reports progress
  // through a task chain and, mirroring that flow, refuses to close mid-transfer
  // rather than pretending a cancel it cannot deliver — the shared composable's
  // stop() only halts client polling; the chain keeps running on the server.
  const transferFolderTarget = ref('');
  const transferFolderMode = ref<BookTransferMode>('copy');

  // The entry is offered only where a folder can actually be transferred: a
  // writable multi-shelf backend (server/desktop) that is not read-only. A
  // reader provider (mobile/pCloud) is not writable, so it never shows.
  const canTransferFolder = computed(() => !readOnly.value && isWritableProvider(getBookshelfProvider()));

  const {
    chain: transferFolderChain,
    status: transferFolderStatus,
    percentage: transferFolderPercentage,
    error: transferFolderError,
    started: transferFolderStarted,
    running: transferFolderRunning,
    finished: transferFolderFinished,
    start: beginTransferFolder,
    reset: resetTransferFolder
  } = useTaskChainProgress({
    onSettled: (status) => {
      if (status === 'failed') {
        return;
      }
      const source = transferFolderTarget.value;
      // A move drops the folder from the source shelf; a copy leaves it. Either
      // way the sidebar's folder tree and per-folder counts have shifted, so pull
      // both stores fresh.
      void Promise.all([fetchFolders(), fetchBooks()]);
      // Only a fully completed move prunes the now-empty source folder, so only
      // then send a user standing in it back to the top. A partial move leaves
      // the books that failed to move — and therefore their source folders — in
      // place, so the current folder is still valid and is where those failures
      // are inspected or retried; keep the user there.
      if (
        status === 'completed' &&
        transferFolderMode.value === 'move' &&
        source &&
        (currentFolder.value === source || currentFolder.value?.startsWith(`${source}/`))
      ) {
        goToFolder(undefined);
      }
    },
    startFailedMessage: () => t('layout.transferFolder.errors.failed'),
    pollFailedMessage: () => t('layout.transferFolder.errors.failed')
  });

  const transferFolderName = computed(() => {
    const segments = transferFolderTarget.value.split('/').filter((segment) => segment.length > 0);
    return segments[segments.length - 1] ?? '';
  });

  function requestTransferFolder(path: string): void {
    if (!canTransferFolder.value) {
      return;
    }
    folderOperationError.value = '';
    transferFolderTarget.value = path;
    resetTransferFolder();
  }

  function cancelTransferFolder(): void {
    // The chain keeps running on the server, so closing mid-transfer would only
    // drop the progress view; refuse it until the transfer settles.
    if (transferFolderRunning.value) {
      return;
    }
    transferFolderTarget.value = '';
    resetTransferFolder();
  }

  // Turns a server refusal into a message the modal can show: the two conflicts
  // become their own readable strings (a book-ID clash lists every colliding ID),
  // and anything else keeps the server's own message.
  function describeTransferFolderError(err: unknown): Error {
    if (err instanceof FolderTransferConflictError) {
      if (err.kind === 'book_id_conflict') {
        return new Error(
          t('layout.transferFolder.errors.conflictBookId', { ids: err.conflictingBookIDs.join(', ') })
        );
      }
      return new Error(t('layout.transferFolder.errors.conflictFolder'));
    }
    return err instanceof Error ? err : new Error(t('layout.transferFolder.errors.failed'));
  }

  async function submitTransferFolder(payload: {
    targetShelfId: string;
    targetParentFolder: string;
    mode: BookTransferMode;
  }): Promise<void> {
    const source = transferFolderTarget.value;
    if (!source || transferFolderStarted.value) {
      return;
    }

    // The folder keeps its own name and nests under the chosen parent, so the full
    // destination path is the parent joined with the folder name (a root parent
    // lands the folder at the target shelf's top level).
    const folderName = transferFolderName.value;
    const parent = payload.targetParentFolder.trim();
    const targetPath = parent ? `${parent}/${folderName}` : folderName;

    transferFolderMode.value = payload.mode;
    // A transfer already in flight for this folder answers with its own chain id,
    // so this attaches to the existing progress instead of scheduling a second.
    await beginTransferFolder(async () => {
      try {
        return await bookshelfWriter().transferFolder(source, payload.targetShelfId, targetPath, payload.mode);
      } catch (err) {
        throw describeTransferFolderError(err);
      }
    });
  }

  function requestDeleteFolder(path: string): void {
    if (readOnly.value) {
      deleteFolderError.value = t('layout.readOnly.writeDisabled');
      return;
    }
    if (deletingFolderMap.value[path]) {
      return;
    }

    deleteFolderError.value = '';
    pendingDeleteFolderPath.value = path;
  }

  function cancelPendingDeleteFolder(): void {
    if (isDeletingPendingFolder.value) {
      return;
    }

    pendingDeleteFolderPath.value = '';
    deleteFolderError.value = '';
  }

  async function confirmDeleteFolder(): Promise<void> {
    const path = pendingDeleteFolderPath.value;
    if (!path || deletingFolderMap.value[path]) {
      return;
    }

    deleteFolderError.value = '';
    deletingFolderMap.value = {
      ...deletingFolderMap.value,
      [path]: true
    };

    try {
      await deleteFolder(path);
      await Promise.all([fetchFolders(), fetchBooks()]);

      if (currentFolder.value === path) {
        goToFolder(undefined);
      }

      pendingDeleteFolderPath.value = '';
    } catch (err) {
      const message = err instanceof Error ? err.message : '';
      if (message === 'Cannot delete this folder because it is not empty.') {
        deleteFolderError.value = t('layout.deleteFolder.notEmpty');
      } else if (message) {
        deleteFolderError.value = message;
      } else {
        deleteFolderError.value = t('layout.deleteFolder.failed');
      }
    } finally {
      const { [path]: _deleted, ...rest } = deletingFolderMap.value;
      deletingFolderMap.value = rest;
    }
  }

  return {
    moveBookError,
    showCreateFolderModal,
    creatingFolder,
    createFolderError,
    deleteFolderError,
    folderOperationError,
    pendingRenameFolderPath,
    renameFolderError,
    deletingFolderMap,
    pendingDeleteFolderPath,
    currentFolder,
    folderTree,
    canOpenFolder,
    isDeletingPendingFolder,
    pendingRenameFolderName,
    isRenamingPendingFolder,
    clearFolderErrors,
    onSelectFolder,
    openCreateFolderModal,
    closeCreateFolderModal,
    onSubmitCreateFolder,
    onMoveBook,
    requestRenameFolder,
    cancelPendingRenameFolder,
    confirmRenameFolder,
    onMoveFolder,
    onOpenFolder,
    canTransferFolder,
    transferFolderTarget,
    transferFolderName,
    transferFolderChain,
    transferFolderStatus,
    transferFolderPercentage,
    transferFolderError,
    transferFolderStarted,
    transferFolderRunning,
    transferFolderFinished,
    requestTransferFolder,
    cancelTransferFolder,
    submitTransferFolder,
    requestDeleteFolder,
    cancelPendingDeleteFolder,
    confirmDeleteFolder
  };
}
