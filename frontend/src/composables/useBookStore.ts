import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import type { Book } from '@/types/book';
import { t } from '@/i18n';

const SHELF_INIT_MAX_AUTO_RETRIES = 10; // ~30s of auto-retry before showing "unreachable"

// Module-level singleton: shared across all components that call useBookStore()
const books = ref<Book[]>([]);
const loading = ref(false);
const error = ref('');
const shelfInitializing = ref(false);
const shelfUnreachable = ref(false);

let retryTimer: ReturnType<typeof setTimeout> | null = null;
let initRetryCount = 0;

function clearRetry(): void {
  if (retryTimer !== null) {
    clearTimeout(retryTimer);
    retryTimer = null;
  }
}

async function fetchBooks(_isAutoRetry = false): Promise<void> {
  clearRetry();
  if (!_isAutoRetry) {
    initRetryCount = 0;
    shelfUnreachable.value = false;
  }
  loading.value = true;
  error.value = '';
  shelfInitializing.value = false;
  try {
    const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER);
    books.value = data.items;
    initRetryCount = 0;
    shelfUnreachable.value = false;
  } catch (err) {
    if (err instanceof ApiError && err.status === 503) {
      initRetryCount++;
      if (initRetryCount >= SHELF_INIT_MAX_AUTO_RETRIES) {
        shelfUnreachable.value = true;
        loading.value = false;
        return;
      }
      shelfInitializing.value = true;
      retryTimer = setTimeout(() => fetchBooks(true), 3000);
      return;
    }
    const msg = err instanceof ApiError && err.isTimeout
      ? t('library.requestTimeout')
      : err instanceof Error ? err.message : t('library.loadFailed');
    error.value = msg;
  } finally {
    if (!shelfInitializing.value) {
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
