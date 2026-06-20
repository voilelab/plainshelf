import { ref } from 'vue';
import { getBookshelfProvider } from '../providers';
import { ApiError } from '../api/client';
import type { Book } from '../types/book';

// Module-level singleton: shared across all components that call useBookStore()
const books = ref<Book[]>([]);
const loading = ref(false);
const error = ref('');
const shelfInitializing = ref(false);

let retryTimer: ReturnType<typeof setTimeout> | null = null;

function clearRetry(): void {
  if (retryTimer !== null) {
    clearTimeout(retryTimer);
    retryTimer = null;
  }
}

async function fetchBooks(search?: string): Promise<void> {
  clearRetry();
  loading.value = true;
  error.value = '';
  shelfInitializing.value = false;
  try {
    const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER, search);
    books.value = data.items;
  } catch (err) {
    if (err instanceof ApiError && err.status === 503) {
      shelfInitializing.value = true;
      retryTimer = setTimeout(() => fetchBooks(search), 3000);
      return;
    }
    const msg = err instanceof ApiError && err.isTimeout
      ? 'Request timed out — the shelf may be slow or unavailable.'
      : err instanceof Error ? err.message : 'Failed to load books';
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
    fetchBooks
  };
}
