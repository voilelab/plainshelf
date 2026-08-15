import { computed } from 'vue';
import type { ComputedRef, Ref } from 'vue';
import { useBookActions } from '@/composables/useBookActions';
import { useWriteAccess } from '@/composables/useWriteAccess';
import type { Book } from '@/types/book';

export interface UseBookCollectionActionsOptions {
  /** The list the emitted ids are resolved against. */
  books: Ref<Book[]> | ComputedRef<Book[]>;
  /** Called after a delete succeeds, to reload the list. */
  onDeleted: () => void;
}

/**
 * Adapts useBookActions to BookCollectionPage, which emits book ids while the
 * actions work on Book objects. Also keeps the read-only guard on delete in one
 * place instead of once per list page.
 */
export function useBookCollectionActions(options: UseBookCollectionActionsOptions) {
  const { writesEnabled } = useWriteAccess();
  const readOnly = computed(() => !writesEnabled.value);
  const actions = useBookActions({ onDeleted: options.onDeleted });

  function findBook(id: string): Book | undefined {
    return options.books.value.find((candidate) => candidate.id === id);
  }

  function onOpenBookFolder(id: string): void {
    void actions.openBookFolder(id);
  }

  function onDownloadBook(id: string): void {
    const book = findBook(id);
    if (book) {
      void actions.downloadBook(book);
    }
  }

  function onRequestDeleteBook(id: string): void {
    if (readOnly.value) {
      return;
    }

    const book = findBook(id);
    if (book) {
      actions.requestDelete(book);
    }
  }

  return {
    ...actions,
    readOnly,
    onOpenBookFolder,
    onDownloadBook,
    onRequestDeleteBook
  };
}
