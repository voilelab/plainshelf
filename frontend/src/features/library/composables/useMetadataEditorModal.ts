import { computed, ref, type ComputedRef, type Ref } from 'vue';
import type { Book } from '@/types/book';

interface UseMetadataEditorModalOptions {
  books: Ref<Book[]>;
  readOnly: ComputedRef<boolean>;
  refresh: () => void | Promise<void>;
}

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
