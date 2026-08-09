import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book } from '@/types/book';

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
    void router.push(`/reader/${id}`);
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
      actionError.value = err instanceof Error ? err.message : 'Failed to open book folder';
    }
  }

  function formatDownloadFilename(book: Book): string {
    const ext = book.format === 'md' ? 'md' : 'txt';
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

      if (provider.saveBookContentToFile) {
        await provider.saveBookContentToFile(book.id, formatDownloadFilename(book));
      } else {
        const blob = await provider.downloadBookContent(book.id);
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = formatDownloadFilename(book);
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.setTimeout(() => URL.revokeObjectURL(url), 5000);
      }
      actionError.value = '';
    } catch (err) {
      actionError.value = err instanceof Error ? err.message : 'Failed to download book';
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
      actionError.value = err instanceof Error ? err.message : 'Failed to delete book';
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
