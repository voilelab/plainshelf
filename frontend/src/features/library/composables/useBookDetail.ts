import { ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import { resolveReadingFormat } from '@/utils/bookFormat';
import { buildMarkdownChapterList, type MarkdownChapterListItem } from '@/utils/markdownChapters';
import type { Book, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { t } from '@/i18n';

/** Chapters are derived from the text, so a broken source only costs the list. */
function readChapters(format: string, content: string | null): MarkdownChapterListItem[] {
  if (format !== 'md' || content === null) {
    return [];
  }

  try {
    return buildMarkdownChapterList(content);
  } catch {
    return [];
  }
}

export function useBookDetail(bookID: () => string) {
  const book = ref<Book | null>(null);
  const progress = ref<ReadingProgress | null>(null);
  const progressContentLength = ref<number | null>(null);
  const currentSource = ref<SourceMeta | null>(null);
  const chapters = ref<MarkdownChapterListItem[]>([]);
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
      // Not worth failing the page over: current_source can name a source that
      // no longer exists, and a book with no source at all has none to
      // describe. It only fills in detail beside the book itself — but its
      // format decides how the text below is read, so it is resolved first.
      const currentSourceData = bookData.current_source
        ? await provider.getSource(currentBookID, bookData.current_source).catch(() => null)
        : null;

      const format = resolveReadingFormat(currentSourceData?.format, bookData.format);
      const needsProgressContentLength =
        progressData.percent === undefined && progressData.char_offset > 0;
      // Read the text the reader will read. The book-scoped route answers from
      // the newest surviving source, which is not always current_source, so
      // going through the source keeps the chapter indexes and the percentage
      // describing the same content the reader opens — with the same fallback
      // the reader uses when that source is gone.
      const bookContent = () =>
        provider.getBookContent(currentBookID).then(({ content }) => content);
      const sourceID = bookData.current_source ?? '';
      // The percentage and the chapter list want the same text, so one load
      // reads the book at most once — and not at all when neither asks for it.
      const contentText =
        needsProgressContentLength || format === 'md'
          ? await (sourceID
              ? provider.getSourceContent(currentBookID, sourceID).catch(bookContent)
              : bookContent()
            ).catch(() => null)
          : null;

      book.value = bookData;
      progress.value = progressData;
      progressContentLength.value =
        needsProgressContentLength && contentText !== null ? contentText.length : null;
      currentSource.value = currentSourceData;
      chapters.value = readChapters(format, contentText);
    } catch (err) {
      book.value = null;
      progress.value = null;
      progressContentLength.value = null;
      currentSource.value = null;
      chapters.value = [];
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
    chapters,
    loading,
    error,
    deleting,
    fetchDetail,
    removeBook
  };
}
