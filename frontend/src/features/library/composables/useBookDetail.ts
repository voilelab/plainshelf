import { ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { t } from '@/i18n';

export function useBookDetail(bookID: () => string) {
  const book = ref<Book | null>(null);
  const progress = ref<ReadingProgress | null>(null);
  const progressContentLength = ref<number | null>(null);
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
      const needsProgressContentLength =
        progressData.percent === undefined && progressData.char_offset > 0;
      const [currentSourceData, currentContentLength] = await Promise.all([
        bookData.current_source
          ? provider.getSource(currentBookID, bookData.current_source)
          : Promise.resolve(null),
        needsProgressContentLength
          ? provider.getBookContent(currentBookID).then(({ content }) => content.length)
          : Promise.resolve(null)
      ]);
      book.value = bookData;
      progress.value = progressData;
      progressContentLength.value = currentContentLength;
      currentSource.value = currentSourceData;
    } catch (err) {
      book.value = null;
      progress.value = null;
      progressContentLength.value = null;
      currentSource.value = null;
      error.value = err instanceof Error ? err.message : t('bookDetail.errors.loadFailed');
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
      error.value = err instanceof Error ? err.message : t('bookDetail.errors.deleteFailed');
      deleting.value = false;
      return false;
    }
  }

  return {
    book,
    progress,
    progressContentLength,
    currentSource,
    loading,
    error,
    deleting,
    fetchDetail,
    removeBook
  };
}
