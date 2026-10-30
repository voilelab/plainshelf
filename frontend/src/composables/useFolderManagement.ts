import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  createFolder,
  deleteFolder,
  FolderTransferConflictError,
  moveFolder,
  NsfwRevealConfirmationError,
  renameFolder,
  setFolderIcon
} from '@/api/folders';
import { FolderMetaConflictError } from '@/api/folderErrors';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { bookshelfWriter } from '@/providers';
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

export function createdFolderDestination(parentPath: string, name: string): string {
  return normalizeFolderPath(`${parentPath}/${name}`);
}

export function useFolderManagement() {
  const route = useRoute();
  const router = useRouter();
  const { t } = useI18n();
  const { books, fetchBooks } = useBookStore();
  const { folders, folderMeta, fetchFolders, fetchFolderMeta } = useFolderStore();
  const { writesEnabled, writeDisabledMessageKey, outgoingCopyEnabled } = useWriteAccess();
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
  const pendingIconFolderPath = ref('');
  const savingFolderIcon = ref(false);
  const folderIconError = ref('');

  const pendingNsfwReveal = ref<{ hiddenBooks: number; retry: () => Promise<void> } | null>(null);

  function askToConfirmReveal(err: unknown, retry: () => Promise<void>): boolean {
    if (!(err instanceof NsfwRevealConfirmationError)) {
      return false;
    }
    pendingNsfwReveal.value = { hiddenBooks: err.hiddenBooks, retry };
    return true;
  }

  function cancelNsfwReveal(): void {
    pendingNsfwReveal.value = null;
  }

  async function confirmNsfwReveal(): Promise<void> {
    const pending = pendingNsfwReveal.value;
    if (!pending) {
      return;
    }
    pendingNsfwReveal.value = null;
    await pending.retry();
  }

  const currentFolder = computed(() => {
    const q = route.query.folders;
    return typeof q === 'string' && q.length > 0 ? q : undefined;
  });

  const folderTree = computed(() => buildFolderTreeNodes(folders.value, folderMeta.value));
  const canOpenFolder = computed(() => Boolean(getPlatformCapabilities().openFolder));
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

  async function onSubmitCreateFolder(payload: { name: string; icon?: string }): Promise<void> {
    if (readOnly.value) {
      createFolderError.value = t(writeDisabledMessageKey.value);
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

      if (payload.icon) {
        try {
          await setFolderIcon(normalized, payload.icon, '');
        } catch (err) {
          const message = err instanceof Error ? err.message : t('layout.folderIcon.failed');
          folderOperationError.value = t('layout.createFolder.iconFailed', { message });
        }
      }
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
      moveBookError.value = t(writeDisabledMessageKey.value);
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
      folderOperationError.value = t(writeDisabledMessageKey.value);
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

  async function confirmRenameFolder(nextName: string, confirmReveal = false): Promise<void> {
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
      await renameFolder(path, nextName, { confirm: confirmReveal });
      await Promise.all([fetchFolders(), fetchBooks()]);

      const destination = renamedFolderDestination(currentFolder.value, path, nextName);
      if (destination !== undefined) {
        goToFolder(destination);
      }

      pendingRenameFolderPath.value = '';
    } catch (err) {
      if (askToConfirmReveal(err, () => confirmRenameFolder(nextName, true))) {
        return;
      }

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

  async function onMoveFolder(
    payload: { folderPath: string; targetFolder: string },
    confirmReveal = false
  ): Promise<void> {
    if (readOnly.value) {
      folderOperationError.value = t(writeDisabledMessageKey.value);
      return;
    }
    folderOperationError.value = '';

    try {
      await moveFolder(payload.folderPath, payload.targetFolder, { confirm: confirmReveal });
      await Promise.all([fetchFolders(), fetchBooks()]);

      const destination = movedFolderDestination(currentFolder.value, payload.folderPath, payload.targetFolder);
      if (destination !== undefined) {
        goToFolder(destination);
      }
    } catch (err) {
      if (askToConfirmReveal(err, () => onMoveFolder(payload, true))) {
        return;
      }

      const message = err instanceof Error ? err.message : '';
      folderOperationError.value = message || t('layout.moveFolder.failed');
    }
  }

  async function onOpenFolder(path: string): Promise<void> {
    folderOperationError.value = '';
    const openDesktopFolder = getPlatformCapabilities().openFolder;
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

  const transferFolderTarget = ref('');
  const transferFolderMode = ref<BookTransferMode>('copy');

  const canTransferFolder = outgoingCopyEnabled;

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
      void Promise.all([fetchFolders(), fetchBooks()]);
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
    if (transferFolderRunning.value) {
      return;
    }
    transferFolderTarget.value = '';
    resetTransferFolder();
  }

  function describeTransferFolderError(err: unknown): Error {
    if (err instanceof NsfwRevealConfirmationError) {
      return new Error(t('layout.folderReveal.transferHeld'));
    }
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

  async function submitTransferFolder(
    payload: {
      targetShelfId: string;
      targetParentFolder: string;
      mode: BookTransferMode;
    },
    confirmReveal = false
  ): Promise<void> {
    const source = transferFolderTarget.value;
    if (!source || transferFolderStarted.value) {
      return;
    }

    const folderName = transferFolderName.value;
    const parent = payload.targetParentFolder.trim();
    const targetPath = parent ? `${parent}/${folderName}` : folderName;

    transferFolderMode.value = payload.mode;
    await beginTransferFolder(async () => {
      try {
        return await bookshelfWriter().transferFolder(source, payload.targetShelfId, targetPath, payload.mode, {
          confirm: confirmReveal
        });
      } catch (err) {
        askToConfirmReveal(err, () => submitTransferFolder(payload, true));
        throw describeTransferFolderError(err);
      }
    });
  }

  const pendingIconFolderName = computed(() => {
    const segments = pendingIconFolderPath.value.split('/').filter((segment) => segment.length > 0);
    return segments[segments.length - 1] ?? '';
  });
  const pendingFolderIcon = computed(() => folderMeta.value[pendingIconFolderPath.value]?.icon ?? '');

  function requestChangeFolderIcon(path: string): void {
    if (readOnly.value) {
      folderOperationError.value = t(writeDisabledMessageKey.value);
      return;
    }
    folderOperationError.value = '';
    folderIconError.value = '';
    pendingIconFolderPath.value = path;
  }

  function cancelChangeFolderIcon(): void {
    if (savingFolderIcon.value) {
      return;
    }
    pendingIconFolderPath.value = '';
    folderIconError.value = '';
  }

  async function confirmChangeFolderIcon(payload: { icon: string }): Promise<void> {
    const path = pendingIconFolderPath.value;
    if (!path || savingFolderIcon.value) {
      return;
    }

    savingFolderIcon.value = true;
    folderIconError.value = '';
    try {
      await setFolderIcon(path, payload.icon, folderMeta.value[path]?.version ?? '');
      await fetchFolderMeta();
      pendingIconFolderPath.value = '';
    } catch (err) {
      if (err instanceof FolderMetaConflictError) {
        await fetchFolderMeta();
        folderIconError.value = t('layout.folderIcon.conflict');
      } else {
        folderIconError.value = (err instanceof Error && err.message) || t('layout.folderIcon.failed');
      }
    } finally {
      savingFolderIcon.value = false;
    }
  }

  function requestDeleteFolder(path: string): void {
    if (readOnly.value) {
      deleteFolderError.value = t(writeDisabledMessageKey.value);
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
    pendingIconFolderPath,
    pendingIconFolderName,
    pendingFolderIcon,
    savingFolderIcon,
    folderIconError,
    requestChangeFolderIcon,
    cancelChangeFolderIcon,
    confirmChangeFolderIcon,
    folderMeta,
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
    pendingNsfwReveal,
    cancelNsfwReveal,
    confirmNsfwReveal,
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
