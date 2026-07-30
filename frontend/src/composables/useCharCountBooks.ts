import { getCurrentInstance, onUnmounted, ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import type { Book } from '@/types/book';

const SHELF_INIT_RETRY_DELAY_MS = 3000;
const SHELF_INIT_MAX_AUTO_RETRIES = 10; // ~30s of auto-retry before giving up

// Books loaded with their char_count, for the few pages that filter or display it.
//
// Deliberately NOT the shared useBookStore singleton: char_count makes the
// backend open every book's current source, so requesting it must stay scoped
// to the page that needs it. Folding an opt-in into useBookStore would make
// that cost depend on which page mounted last, and any later fetchBooks() from
// another page would silently strip char_count back off the shared list.
// Each call returns its own refs, so mounting this page never changes what
// other pages fetch.
export function useCharCountBooks() {
  const books = ref<Book[]>([]);
  const loading = ref(false);
  const error = ref('');

  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let initRetryCount = 0;

  function clearRetry(): void {
    if (retryTimer !== null) {
      clearTimeout(retryTimer);
      retryTimer = null;
    }
  }

  async function fetchBooks(isAutoRetry = false): Promise<void> {
    clearRetry();
    if (!isAutoRetry) {
      initRetryCount = 0;
    }
    loading.value = true;
    error.value = '';
    try {
      const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER, {
        includeCharCount: true
      });
      books.value = data.items;
      initRetryCount = 0;
    } catch (err) {
      // A shelf that is still initializing answers 503 for a while; keep the
      // page in its loading state and retry, matching useBookStore, instead of
      // stranding the user on an error the shelf resolves on its own.
      if (err instanceof ApiError && err.status === 503) {
        initRetryCount++;
        if (initRetryCount < SHELF_INIT_MAX_AUTO_RETRIES) {
          retryTimer = setTimeout(() => void fetchBooks(true), SHELF_INIT_RETRY_DELAY_MS);
          return;
        }
        error.value = 'The shelf is still starting up and did not become ready.';
      } else {
        error.value = err instanceof ApiError && err.isTimeout
          ? 'Request timed out — the shelf may be slow or unavailable.'
          : err instanceof Error ? err.message : 'Failed to load books';
      }
    } finally {
      if (retryTimer === null) {
        loading.value = false;
      }
    }
  }

  // These refs are page-scoped, so a pending retry must not outlive the page.
  if (getCurrentInstance()) {
    onUnmounted(clearRetry);
  }

  return {
    books,
    loading,
    error,
    fetchBooks
  };
}
