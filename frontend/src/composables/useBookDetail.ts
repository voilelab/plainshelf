import { ref } from 'vue';
import {
  deleteBook,
  getBook,
  getReadingProgress
} from '../api/books';
import type { Book, ReadingProgress } from '../types/book';

export function useBookDetail(bookID: () => string) {
  const book = ref<Book | null>(null);
  const progress = ref<ReadingProgress | null>(null);
  const loading = ref(false);
  const error = ref('');
  const deleting = ref(false);

  async function fetchDetail(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      const [bookData, progressData] = await Promise.all([
        getBook(bookID()),
        getReadingProgress(bookID())
      ]);
      book.value = bookData;
      progress.value = progressData;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load detail';
    } finally {
      loading.value = false;
    }
  }

  async function removeBook(targetBookID?: string): Promise<boolean> {
    deleting.value = true;
    error.value = '';
    try {
      await deleteBook(targetBookID ?? bookID());
      return true;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to delete book';
      deleting.value = false;
      return false;
    }
  }

  return {
    book,
    progress,
    loading,
    error,
    deleting,
    fetchDetail,
    removeBook
  };
}
