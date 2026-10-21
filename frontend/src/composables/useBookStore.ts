import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import type { Book } from '@/types/book';
import { t } from '@/i18n';

const books = ref<Book[]>([]);
const loading = ref(false);
const error = ref('');
const shelfInitializing = ref(false);
const shelfUnreachable = ref(false);

const initRetry = createShelfInitRetry();

async function fetchBooks(_isAutoRetry = false): Promise<void> {
  if (_isAutoRetry) {
    initRetry.cancel();
  } else {
    initRetry.reset();
    shelfUnreachable.value = false;
  }
  loading.value = true;
  error.value = '';
  shelfInitializing.value = false;
  try {
    const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER);
    books.value = data.items;
    initRetry.reset();
    shelfInitializing.value = false;
    shelfUnreachable.value = false;
  } catch (err) {
    if (isShelfInitializing(err)) {
      if (initRetry.schedule(() => void fetchBooks(true), err)) {
        shelfInitializing.value = true;
        return;
      }
      shelfUnreachable.value = true;
      loading.value = false;
      return;
    }
    const msg = err instanceof ApiError && err.isTimeout
      ? t('library.requestTimeout')
      : err instanceof Error ? err.message : t('library.loadFailed');
    error.value = msg;
  } finally {
    if (!initRetry.pending) {
      loading.value = false;
    }
  }
}

export function useBookStore() {
  return {
    books,
    loading,
    error,
    shelfInitializing,
    shelfUnreachable,
    fetchBooks
  };
}
