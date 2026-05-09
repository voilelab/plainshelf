import { nextTick, ref } from 'vue';
import { getBook, getBookContent, getReadingProgress, saveBookmark } from '../api/books';
import type { ReadingProgress } from '../types/book';

export function useReader(bookID: () => string) {
  const title = ref('');
  const content = ref('');
  const progress = ref<ReadingProgress | null>(null);
  const loading = ref(false);
  const bookmarking = ref(false);
  const error = ref('');
  const currentOffset = ref(0);
  const readerRef = ref<HTMLDivElement | null>(null);

  function normalizeProgress(next: ReadingProgress): ReadingProgress {
    const total = content.value.length;
    const percent =
      next.percent ??
      (total > 0 ? Math.max(0, Math.min(100, Math.round((next.char_offset / total) * 100))) : 0);

    return {
      ...next,
      percent
    };
  }

  async function syncScrollToProgress(next: ReadingProgress): Promise<void> {
    await nextTick();

    const el = readerRef.value;
    const total = content.value.length;
    if (!el || total <= 0) {
      return;
    }

    const maxScrollTop = el.scrollHeight - el.clientHeight;
    if (maxScrollTop <= 0) {
      return;
    }

    const ratio = Math.max(0, Math.min(1, next.char_offset / total));
    el.scrollTop = Math.round(maxScrollTop * ratio);
  }

  async function fetchReaderData(): Promise<void> {
    loading.value = true;
    error.value = '';
    let restoredProgress: ReadingProgress | null = null;

    try {
      const [book, bookContent, currentProgress] = await Promise.all([
        getBook(bookID()),
        getBookContent(bookID()),
        getReadingProgress(bookID())
      ]);
      title.value = book.title;
      content.value = bookContent.content;
      progress.value = normalizeProgress(currentProgress);
      currentOffset.value = currentProgress.char_offset;
      restoredProgress = currentProgress;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load reader data';
    } finally {
      loading.value = false;
    }

    if (restoredProgress) {
      await syncScrollToProgress(restoredProgress);
    }
  }

  function onScroll(): void {
    const el = readerRef.value;
    if (!el) {
      return;
    }
    const max = el.scrollHeight - el.clientHeight;
    const ratio = max > 0 ? el.scrollTop / max : 0;
    const nextPercent = Math.max(0, Math.min(100, Math.round(ratio * 100)));
    const nextOffset = Math.round(content.value.length * ratio);
    currentOffset.value = nextOffset;

    if (progress.value) {
      progress.value = {
        ...progress.value,
        char_offset: nextOffset,
        percent: nextPercent
      };
    }
  }

  async function bookmarkCurrent(): Promise<void> {
    bookmarking.value = true;
    error.value = '';
    try {
      await saveBookmark(bookID(), { char_offset: currentOffset.value });
      const nextProgress = await getReadingProgress(bookID());
      progress.value = normalizeProgress(nextProgress);
      currentOffset.value = nextProgress.char_offset;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to create bookmark';
    } finally {
      bookmarking.value = false;
    }
  }

  return {
    title,
    content,
    progress,
    loading,
    bookmarking,
    error,
    readerRef,
    fetchReaderData,
    onScroll,
    bookmarkCurrent
  };
}
