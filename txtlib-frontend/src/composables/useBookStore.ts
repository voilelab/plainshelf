import { ref } from 'vue';
import { listBooks } from '../api/books';
import type { Book } from '../types/book';

// Module-level singleton: shared across all components that call useBookStore()
const books = ref<Book[]>([]);
const loading = ref(false);
const error = ref('');

async function fetchBooks(search?: string): Promise<void> {
  loading.value = true;
  error.value = '';
  try {
    const data = await listBooks(1, Number.MAX_SAFE_INTEGER, search);
    books.value = data.items;
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load books';
  } finally {
    loading.value = false;
  }
}

export function useBookStore() {
  return {
    books,
    loading,
    error,
    fetchBooks
  };
}
