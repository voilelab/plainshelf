import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import type { Book } from '@/types/book';
import { t } from '@/i18n';

// Module-level singleton: shared across all components that call useBookStore()
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
    // Also clears a retry an overlapping load armed: the shelf answered, so
    // that retry has nothing left to wait for.
    initRetry.reset();
    shelfInitializing.value = false;
    shelfUnreachable.value = false;
  } catch (err) {
    if (isShelfInitializing(err)) {
      if (initRetry.schedule(() => void fetchBooks(true))) {
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
    // A pending retry keeps the page in its loading state rather than flashing
    // an error between attempts. Keyed on the retry itself, not on the
    // initializing flag, which an overlapping load can leave set with no retry
    // behind it.
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
