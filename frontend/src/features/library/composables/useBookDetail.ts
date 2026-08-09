import { ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';

export function useBookDetail(bookID: () => string) {
  const book = ref<Book | null>(null);
  const progress = ref<ReadingProgress | null>(null);
  const currentSource = ref<SourceMeta | null>(null);
  const loading = ref(false);
  const error = ref('');
  const deleting = ref(false);

  async function fetchDetail(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      const currentBookID = bookID();
      const provider = getBookshelfProvider();
      const [bookData, progressData] = await Promise.all([
        provider.getBook(currentBookID),
        provider.getReadProgress(currentBookID)
      ]);
      const currentSourceData = bookData.current_source
        ? await provider.getSource(currentBookID, bookData.current_source)
        : null;
      book.value = bookData;
      progress.value = progressData;
      currentSource.value = currentSourceData;
    } catch (err) {
      book.value = null;
      progress.value = null;
      currentSource.value = null;
      error.value = err instanceof Error ? err.message : 'Failed to load detail';
    } finally {
      loading.value = false;
    }
  }

  async function removeBook(): Promise<boolean> {
    deleting.value = true;
    error.value = '';
    try {
      await bookshelfWriter().deleteBook(bookID());
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
    currentSource,
    loading,
    error,
    deleting,
    fetchDetail,
    removeBook
  };
}
