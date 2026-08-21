import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useBookStore } from '@/composables/useBookStore';
import { useLayerStore } from '@/composables/useLayerStore';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book } from '@/types/book';
import { getLayerPath, layerPathEquals } from '@/utils/layers';
import { t } from '@/i18n';

export interface UseBookActionsOptions {
  onDeleted?: (book: Book) => void;
  onMoved?: (book: Book, targetLayer: string) => void;
  onCopied?: (copy: Book, targetLayer: string) => void;
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
 * download / move to another layer / delete) used by both BookDetailPage and
 * the card view's context menu. Each call site gets its own instance — the
 * delete/download busy state below is intentionally per-instance, not a module
 * singleton.
 */
export function useBookActions(options: UseBookActionsOptions = {}) {
  const router = useRouter();
  const { layers, loaded: layersLoaded, fetchLayers } = useLayerStore();
  const { fetchBooks } = useBookStore();

  const downloading = ref(false);
  const actionError = ref('');
  const deleteTarget = ref<Book | null>(null);
  const deleting = ref(false);
  const moveTarget = ref<Book | null>(null);
  const moving = ref(false);
  const copyTarget = ref<Book | null>(null);
  const copying = ref(false);

  /**
   * Destinations offered for `moveTarget`, flat and sorted like the batch move
   * modal's list. The book's own layer is dropped because moving there is a
   * no-op; the root entry is not in here at all — MoveBooksModal renders its
   * own labelled root option.
   */
  const moveLayerOptions = computed(() => {
    const currentLayer = moveTarget.value ? getLayerPath(moveTarget.value) : '';
    return [...new Set(layers.value.filter((layer) => layer && layer !== '/'))]
      .filter((layer) => !layerPathEquals(layer, currentLayer))
      .sort();
  });

  /**
   * Destinations offered for `copyTarget`. Unlike moveLayerOptions this keeps the
   * book's own layer: copying a book into the layer it already sits in is a valid
   * "duplicate here", not a no-op. The root is offered by the modal itself.
   */
  const copyLayerOptions = computed(() =>
    [...new Set(layers.value.filter((layer) => layer && layer !== '/'))].sort()
  );

  const canOpenBookFolder = computed(() => Boolean(getBookshelfProvider().openDesktopBookFolder));

  /**
   * Opens the reader, optionally at one chapter instead of the saved progress.
   * The index is the reader's own section index, so it survives a title change.
   */
  function goRead(id: string, sectionIndex?: number): void {
    if (typeof sectionIndex === 'number' && Number.isFinite(sectionIndex)) {
      void router.push({ path: `/reader/${id}`, query: { section: String(Math.trunc(sectionIndex)) } });
      return;
    }

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

  function requestMove(book: Book): void {
    moveTarget.value = book;
    // The move dialog shows actionError itself, so it must not open carrying
    // the message some earlier action left behind.
    actionError.value = '';
    // The sidebar normally fills the layer store on load, but the detail page
    // can be opened directly by URL, and mobile has no sidebar at all.
    if (!layersLoaded.value) {
      void fetchLayers();
    }
  }

  function cancelMove(): void {
    if (!moving.value) {
      moveTarget.value = null;
    }
  }

  async function submitMove(targetLayer: string): Promise<void> {
    const target = moveTarget.value;
    if (!target || moving.value) {
      return;
    }

    // moveLayerOptions already drops the book's own layer, but the modal always
    // offers the root entry, so a book at the root can still land here.
    if (layerPathEquals(getLayerPath(target), targetLayer)) {
      moveTarget.value = null;
      return;
    }

    moving.value = true;
    actionError.value = '';

    try {
      await bookshelfWriter().updateBookLayer(target.id, targetLayer);
      moveTarget.value = null;
      // The sidebar derives its per-layer counts from the book store, the same
      // refresh the drag-to-layer path performs after a move.
      void fetchBooks();
      options.onMoved?.(target, targetLayer);
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
    if (!layersLoaded.value) {
      void fetchLayers();
    }
  }

  function cancelCopy(): void {
    if (!copying.value) {
      copyTarget.value = null;
    }
  }

  async function submitCopy(targetLayer: string): Promise<void> {
    const target = copyTarget.value;
    if (!target || copying.value) {
      return;
    }

    copying.value = true;
    actionError.value = '';

    try {
      const copy = await bookshelfWriter().copyBook(target.id, targetLayer);
      copyTarget.value = null;
      // Keep the sidebar's per-layer counts in step with the new book, the same
      // refresh submitMove performs.
      void fetchBooks();
      options.onCopied?.(copy, targetLayer);
    } catch (err) {
      // Leaves the modal open so the destination stays picked for a retry.
      actionError.value = err instanceof Error ? err.message : t('bookDetail.errors.copyFailed');
    } finally {
      copying.value = false;
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
    moveTarget,
    moving,
    moveLayerOptions,
    copyTarget,
    copying,
    copyLayerOptions,
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
