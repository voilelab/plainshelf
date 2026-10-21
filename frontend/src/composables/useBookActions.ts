import { sortFolderPaths } from '@/utils/folders';
import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useToasts } from '@/composables/useToasts';
import { useReaderLaunch } from '@/composables/useReaderLaunch';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { BookTransferMode } from '@/api/books';
import type { Book } from '@/types/book';
import type { TaskStatus } from '@/types/task';
import { getFolderPath, folderPathEquals } from '@/utils/folders';
import { t } from '@/i18n';

interface UseBookActionsOptions {
  onDeleted?: (book: Book) => void;
  onMoved?: (book: Book, targetFolder: string) => void;
  onCopied?: (copy: Book, targetFolder: string) => void;
  onTransferred?: (book: Book, mode: BookTransferMode, status: TaskStatus) => void;
}

export const DELETE_BOOK_DESCRIPTION =
  'The book will be moved to Trash. You can restore it later.';

function sanitizeDownloadName(name: string): string {
  return name
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, ' ')
    .trim() || 'book';
}

export function useBookActions(options: UseBookActionsOptions = {}) {
  const router = useRouter();
  const { folders, folderMeta: metadata, loaded: foldersLoaded, fetchFolders } = useFolderStore();
  const { fetchBooks } = useBookStore();
  const { showToast } = useToasts();
  const { launchReader } = useReaderLaunch();

  const downloading = ref(false);
  const actionError = ref('');
  const deleteTarget = ref<Book | null>(null);
  const deleting = ref(false);
  const moveTarget = ref<Book | null>(null);
  const moving = ref(false);
  const copyTarget = ref<Book | null>(null);
  const copying = ref(false);
  const transferTarget = ref<Book | null>(null);
  const transferMode = ref<BookTransferMode>('copy');

  const {
    status: transferStatus,
    percentage: transferPercentage,
    error: transferError,
    started: transferStarted,
    running: transferring,
    finished: transferFinished,
    start: beginTransfer,
    reset: resetTransfer
  } = useTaskChainProgress({
    onSettled: (status) => {
      if (status === 'failed') {
        return;
      }
      const target = transferTarget.value;
      void fetchBooks();
      if (target) {
        options.onTransferred?.(target, transferMode.value, status);
      }
    },
    startFailedMessage: () => t('bookDetail.errors.transferFailed'),
    pollFailedMessage: () => t('bookDetail.errors.transferPollFailed')
  });

  const moveFolderOptions = computed(() => {
    const currentFolder = moveTarget.value ? getFolderPath(moveTarget.value) : '';
    return sortFolderPaths([...new Set(folders.value.filter((folder) => folder && folder !== '/'))]
      .filter((folder) => !folderPathEquals(folder, currentFolder)).sort(), metadata?.value ?? {});
  });

  const copyFolderOptions = computed(() =>
    sortFolderPaths([...new Set(folders.value.filter((folder) => folder && folder !== '/'))].sort(), metadata?.value ?? {})
  );

  const canOpenBookFolder = computed(() => Boolean(getPlatformCapabilities().openBookFolder));

  function goRead(id: string, sectionIndex?: number): void {
    launchReader(id, sectionIndex);
  }

  function openDetail(id: string): void {
    void router.push(`/books/${id}`);
  }

  async function openBookFolder(id: string): Promise<void> {
    try {
      await getPlatformCapabilities().openBookFolder?.(id);
      actionError.value = '';
    } catch (err) {
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.openFolderFailed');
    }
  }

  function formatDownloadFilename(book: Book, sourceFormat?: string): string {
    const ext = (sourceFormat ?? book.format) === 'md' ? 'md' : 'txt';
    return `${sanitizeDownloadName(book.title || book.id)}.${ext}`;
  }

  async function downloadBook(book: Book): Promise<void> {
    if (downloading.value) {
      return;
    }

    downloading.value = true;
    actionError.value = '';

    try {
      const provider = getBookshelfProvider();
      const sourceFormat = book.current_source
        ? await provider.getSource(book.id, book.current_source).then((source) => source.format).catch(() => undefined)
        : undefined;

      const platform = getPlatformCapabilities();
      if (platform.saveBookContentToFile) {
        const location = await platform.saveBookContentToFile(
          book.id,
          formatDownloadFilename(book, sourceFormat)
        );
        if (typeof location === 'string' && location) {
          showToast(t('bookDetail.messages.exported', { location }));
        }
      } else {
        const blob = await provider.downloadBookContent(book.id);
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = formatDownloadFilename(book, sourceFormat);
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.setTimeout(() => URL.revokeObjectURL(url), 5000);
      }
      actionError.value = '';
    } catch (err) {
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.downloadFailed');
    } finally {
      downloading.value = false;
    }
  }

  function requestMove(book: Book): void {
    moveTarget.value = book;
    actionError.value = '';
    if (!foldersLoaded.value) {
      void fetchFolders();
    }
  }

  function cancelMove(): void {
    if (!moving.value) {
      moveTarget.value = null;
    }
  }

  async function submitMove(targetFolder: string): Promise<void> {
    const target = moveTarget.value;
    if (!target || moving.value) {
      return;
    }

    if (folderPathEquals(getFolderPath(target), targetFolder)) {
      moveTarget.value = null;
      return;
    }

    moving.value = true;
    actionError.value = '';

    try {
      await bookshelfWriter().updateBookFolder(target.id, targetFolder);
      moveTarget.value = null;
      void fetchBooks();
      options.onMoved?.(target, targetFolder);
    } catch (err) {
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.moveFailed');
    } finally {
      moving.value = false;
    }
  }

  function requestCopy(book: Book): void {
    copyTarget.value = book;
    actionError.value = '';
    if (!foldersLoaded.value) {
      void fetchFolders();
    }
  }

  function cancelCopy(): void {
    if (!copying.value) {
      copyTarget.value = null;
    }
  }

  async function submitCopy(targetFolder: string): Promise<void> {
    const target = copyTarget.value;
    if (!target || copying.value) {
      return;
    }

    copying.value = true;
    actionError.value = '';

    try {
      const copy = await bookshelfWriter().copyBook(target.id, targetFolder);
      copyTarget.value = null;
      void fetchBooks();
      options.onCopied?.(copy, targetFolder);
    } catch (err) {
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.copyFailed');
    } finally {
      copying.value = false;
    }
  }

  function requestTransfer(book: Book): void {
    transferTarget.value = book;
    actionError.value = '';
    resetTransfer();
  }

  function cancelTransfer(): void {
    if (transferring.value) {
      return;
    }
    transferTarget.value = null;
    resetTransfer();
  }

  async function submitTransfer(payload: {
    targetShelfId: string;
    targetFolder: string;
    mode: BookTransferMode;
  }): Promise<void> {
    const target = transferTarget.value;
    if (!target || transferStarted.value) {
      return;
    }

    transferMode.value = payload.mode;
    await beginTransfer(() =>
      bookshelfWriter().transferBook(target.id, payload.targetShelfId, payload.targetFolder, payload.mode)
    );
  }

  function requestDelete(book: Book): void {
    deleteTarget.value = book;
  }

  function cancelDelete(): void {
    deleteTarget.value = null;
  }

  async function confirmDelete(): Promise<void> {
    const target = deleteTarget.value;
    if (!target || deleting.value) {
      return;
    }

    deleting.value = true;
    actionError.value = '';

    try {
      await bookshelfWriter().deleteBook(target.id);
      deleting.value = false;
      deleteTarget.value = null;
      options.onDeleted?.(target);
    } catch (err) {
      deleting.value = false;
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.deleteFailed');
    }
  }

  function dismissActionError(): void {
    actionError.value = '';
  }

  return {
    downloading,
    actionError,
    deleteTarget,
    deleting,
    moveTarget,
    moving,
    moveFolderOptions,
    copyTarget,
    copying,
    copyFolderOptions,
    transferTarget,
    transferMode,
    transferStatus,
    transferPercentage,
    transferError,
    transferStarted,
    transferring,
    transferFinished,
    requestTransfer,
    cancelTransfer,
    submitTransfer,
    canOpenBookFolder,
    goRead,
    openDetail,
    openBookFolder,
    downloadBook,
    requestMove,
    cancelMove,
    submitMove,
    requestCopy,
    cancelCopy,
    submitCopy,
    requestDelete,
    cancelDelete,
    confirmDelete,
    dismissActionError
  };
}
