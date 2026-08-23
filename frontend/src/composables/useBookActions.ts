import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { useToasts } from '@/composables/useToasts';
import { bookshelfWriter, getBookshelfProvider, isWebRuntime } from '@/providers';
import type { BookTransferMode } from '@/api/books';
import type { Book } from '@/types/book';
import type { TaskStatus } from '@/types/task';
import { getFolderPath, folderPathEquals } from '@/utils/folders';
import { t } from '@/i18n';

export interface UseBookActionsOptions {
  onDeleted?: (book: Book) => void;
  onMoved?: (book: Book, targetFolder: string) => void;
  onCopied?: (copy: Book, targetFolder: string) => void;
  /**
   * Fired once a cross-shelf transfer settles without failing. `status` is the
   * chain's terminal status so a caller can distinguish a full completion from a
   * partial one; the modal still shows the outcome, so this is for side effects
   * like navigating away after a move.
   */
  onTransferred?: (book: Book, mode: BookTransferMode, status: TaskStatus) => void;
}

/** Shown by every DeleteModal that moves a book to Trash. */
export const DELETE_BOOK_DESCRIPTION =
  'The book will be moved to Trash. You can restore it later.';

function sanitizeDownloadName(name: string): string {
  return name
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, ' ')
    .trim() || 'book';
}

/**
 * Shared book-level actions (read / open detail / edit / open book folder /
 * download / move to another folder / delete) used by both BookDetailPage and
 * the card view's context menu. Each call site gets its own instance — the
 * delete/download busy state below is intentionally per-instance, not a module
 * singleton.
 */
export function useBookActions(options: UseBookActionsOptions = {}) {
  const router = useRouter();
  const { folders, loaded: foldersLoaded, fetchFolders } = useFolderStore();
  const { fetchBooks } = useBookStore();
  const { showToast } = useToasts();

  const downloading = ref(false);
  const actionError = ref('');
  const deleteTarget = ref<Book | null>(null);
  const deleting = ref(false);
  const moveTarget = ref<Book | null>(null);
  const moving = ref(false);
  const copyTarget = ref<Book | null>(null);
  const copying = ref(false);
  const transferTarget = ref<Book | null>(null);
  // The mode of the transfer currently in flight, read when it settles so the
  // caller learns whether the source was moved away or merely copied.
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
      // A failed transfer changed nothing on the active shelf and must not
      // trigger the post-success side effects (a navigation away, say).
      if (status === 'failed') {
        return;
      }
      const target = transferTarget.value;
      // A move drops the source from the active shelf and a copy leaves it, but
      // either way the sidebar's per-folder counts have shifted, so refresh the
      // book store — the same refresh submitMove/submitCopy perform.
      void fetchBooks();
      if (target) {
        options.onTransferred?.(target, transferMode.value, status);
      }
    },
    startFailedMessage: () => t('bookDetail.errors.transferFailed'),
    pollFailedMessage: () => t('bookDetail.errors.transferPollFailed')
  });

  /**
   * Destinations offered for `moveTarget`, flat and sorted like the batch move
   * modal's list. The book's own folder is dropped because moving there is a
   * no-op; the root entry is not in here at all — MoveBooksModal renders its
   * own labelled root option.
   */
  const moveFolderOptions = computed(() => {
    const currentFolder = moveTarget.value ? getFolderPath(moveTarget.value) : '';
    return [...new Set(folders.value.filter((folder) => folder && folder !== '/'))]
      .filter((folder) => !folderPathEquals(folder, currentFolder))
      .sort();
  });

  /**
   * Destinations offered for `copyTarget`. Unlike moveFolderOptions this keeps the
   * book's own folder: copying a book into the folder it already sits in is a valid
   * "duplicate here", not a no-op. The root is offered by the modal itself.
   */
  const copyFolderOptions = computed(() =>
    [...new Set(folders.value.filter((folder) => folder && folder !== '/'))].sort()
  );

  const canOpenBookFolder = computed(() => Boolean(getBookshelfProvider().openDesktopBookFolder));

  /**
   * Opens the reader, optionally at one chapter instead of the saved progress.
   * The index is the reader's own section index, so it survives a title change.
   */
  function goRead(id: string, sectionIndex?: number): void {
    const hasSection = typeof sectionIndex === 'number' && Number.isFinite(sectionIndex);
    const to = hasSection
      ? { path: `/reader/${id}`, query: { section: String(Math.trunc(sectionIndex)) } }
      : { path: `/reader/${id}` };

    // On a plain web-server build the reader opens in a new tab so the library
    // or book page it was launched from is not replaced wholesale — closing the
    // tab returns the reader there. The desktop, mobile and reader shells keep
    // the in-place SPA navigation. Matches externalLinks.ts's window.open idiom.
    //
    // No router.push fallback here: with noopener/noreferrer window.open returns
    // null even on success (per the HTML spec), so its result cannot tell a
    // blocked pop-up from an opened one. Pushing "on failure" would therefore
    // fire on every success too and navigate the original tab as well, which is
    // exactly the wholesale replacement this feature avoids. A user-gesture open
    // is not pop-up-blocked in practice.
    if (isWebRuntime()) {
      window.open(router.resolve(to).href, '_blank', 'noopener,noreferrer');
      return;
    }

    // On the desktop app reading opens the standalone reader in its own window;
    // its progress flows back into this library (reading_progress.json). Only the
    // desktop provider defines openDesktopReader, so web/mobile fall through. A
    // chapter jump passes its section so the reader opens on that chapter — the
    // same window as the default read, not the in-app one — and the in-app
    // /reader/:id route stays the fallback if the reader will not launch (e.g. not
    // installed).
    const provider = getBookshelfProvider();
    if (provider.openDesktopReader) {
      provider.openDesktopReader(id, hasSection ? Math.trunc(sectionIndex) : undefined).catch(() => {
        void router.push(to);
      });
      return;
    }

    void router.push(to);
  }

  function openDetail(id: string): void {
    void router.push(`/books/${id}`);
  }

  function goEdit(id: string): void {
    void router.push(`/books/${id}/edit`);
  }

  async function openBookFolder(id: string): Promise<void> {
    try {
      await getBookshelfProvider().openDesktopBookFolder?.(id);
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

      if (provider.saveBookContentToFile) {
        // A returned location string means the file was written somewhere with
        // no save dialog of its own (the mobile Documents export), so confirm it
        // with a toast; the desktop dialog resolves to void and needs none.
        const location = await provider.saveBookContentToFile(
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
    // The move dialog shows actionError itself, so it must not open carrying
    // the message some earlier action left behind.
    actionError.value = '';
    // The sidebar normally fills the folder store on load, but the detail page
    // can be opened directly by URL, and mobile has no sidebar at all.
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

    // moveFolderOptions already drops the book's own folder, but the modal always
    // offers the root entry, so a book at the root can still land here.
    if (folderPathEquals(getFolderPath(target), targetFolder)) {
      moveTarget.value = null;
      return;
    }

    moving.value = true;
    actionError.value = '';

    try {
      await bookshelfWriter().updateBookFolder(target.id, targetFolder);
      moveTarget.value = null;
      // The sidebar derives its per-folder counts from the book store, the same
      // refresh the drag-to-folder path performs after a move.
      void fetchBooks();
      options.onMoved?.(target, targetFolder);
    } catch (err) {
      // Leaves the modal open so the destination stays picked for a retry.
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.moveFailed');
    } finally {
      moving.value = false;
    }
  }

  function requestCopy(book: Book): void {
    copyTarget.value = book;
    // The copy dialog shows actionError itself, so it must not open carrying a
    // message some earlier action left behind.
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
      // Keep the sidebar's per-folder counts in step with the new book, the same
      // refresh submitMove performs.
      void fetchBooks();
      options.onCopied?.(copy, targetFolder);
    } catch (err) {
      // Leaves the modal open so the destination stays picked for a retry.
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.copyFailed');
    } finally {
      copying.value = false;
    }
  }

  function requestTransfer(book: Book): void {
    transferTarget.value = book;
    // The transfer dialog surfaces its own progress and errors, so it must not
    // open carrying a message some earlier action left behind.
    actionError.value = '';
    resetTransfer();
  }

  function cancelTransfer(): void {
    // Closing mid-transfer would drop the only view of progress the user has,
    // while the task keeps running on the server regardless; mirror the
    // empty-trash modal and refuse to close until it settles.
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
    // A transfer already in flight for this book answers with its own chain id,
    // so this attaches to the existing progress instead of scheduling a second.
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
    goEdit,
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
