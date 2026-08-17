import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { isDesktopRuntime, openDesktopReaderWindow } from '@/api/desktop';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book } from '@/types/book';
import { t } from '@/i18n';

export interface UseBookActionsOptions {
  onDeleted?: (book: Book) => void;
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
 * download / delete) used by both BookDetailPage and the card view's
 * context menu. Each call site gets its own instance — the delete/download
 * busy state below is intentionally per-instance, not a module singleton.
 */
export function useBookActions(options: UseBookActionsOptions = {}) {
  const router = useRouter();

  const downloading = ref(false);
  const actionError = ref('');
  const deleteTarget = ref<Book | null>(null);
  const deleting = ref(false);

  const canOpenBookFolder = computed(() => Boolean(getBookshelfProvider().openDesktopBookFolder));

  function goRead(id: string): void {
    if (isDesktopRuntime()) {
      if (openDesktopReaderWindow(id)) {
        return;
      }
      void router.push(`/reader/${id}`);
      return;
    }
    window.open(`/reader/${id}`, '_blank');
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
        await provider.saveBookContentToFile(book.id, formatDownloadFilename(book, sourceFormat));
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
    canOpenBookFolder,
    goRead,
    openDetail,
    goEdit,
    openBookFolder,
    downloadBook,
    requestDelete,
    cancelDelete,
    confirmDelete,
    dismissActionError
  };
}
