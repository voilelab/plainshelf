import { ref } from 'vue';
import { listBooks } from '../api/books';
import type { Book } from '../types/book';

export function useBooks() {
  const books = ref<Book[]>([]);
  const loading = ref(false);
  const error = ref('');
  const pageSize = ref(8);

  async function fetchBooks(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      const data = await listBooks(1, Number.MAX_SAFE_INTEGER);
      books.value = data.items;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load books';
    } finally {
      loading.value = false;
    }
  }

  return {
    books,
    loading,
    error,
    pageSize,
    fetchBooks
  };
}
