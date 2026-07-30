import { ref } from 'vue';
import { getBookshelfProvider } from '../providers';
import { ApiError } from '../api/client';
import type { Book } from '../types/book';

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

  async function fetchBooks(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER, {
        includeCharCount: true
      });
      books.value = data.items;
    } catch (err) {
      error.value = err instanceof ApiError && err.isTimeout
        ? 'Request timed out — the shelf may be slow or unavailable.'
        : err instanceof Error ? err.message : 'Failed to load books';
    } finally {
      loading.value = false;
    }
  }

  return {
    books,
    loading,
    error,
    fetchBooks
  };
}
