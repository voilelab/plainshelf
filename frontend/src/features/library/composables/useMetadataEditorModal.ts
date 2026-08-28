import { computed, ref, type ComputedRef, type Ref } from 'vue';
import type { Book } from '@/types/book';

export interface UseMetadataEditorModalOptions {
  books: Ref<Book[]>;
  readOnly: ComputedRef<boolean>;
  refresh: () => void | Promise<void>;
}

/**
 * Owns the list-page side of the metadata modal lifecycle. The modal itself
 * owns loading and saving; this keeps a stable book id selected while a list is
 * reloaded, blocks forged edit events in read-only mode, and reconciles the
 * saved result with both the current view and the provider's latest listing.
 */
export function useMetadataEditorModal(options: UseMetadataEditorModalOptions) {
  const selectedBookId = ref<string | null>(null);
  const open = computed(() => selectedBookId.value !== null && !options.readOnly.value);

  function openEditor(id: string): void {
    if (
      options.readOnly.value
      || selectedBookId.value !== null
      || !options.books.value.some((book) => book.id === id)
    ) {
      return;
    }
    selectedBookId.value = id;
  }

  function closeEditor(): void {
    selectedBookId.value = null;
  }

  function onSaved(updatedBook: Book): void {
    // Patch first: refresh failures leave a truthful card/history entry. The
    // full refresh still runs so filters, sorting, folders, and pagination are
    // evaluated against the provider's complete current listing.
    options.books.value = options.books.value.map((book) =>
      book.id === updatedBook.id ? updatedBook : book
    );
    void options.refresh();
  }

  return {
    selectedBookId,
    open,
    openEditor,
    closeEditor,
    onSaved
  };
}
