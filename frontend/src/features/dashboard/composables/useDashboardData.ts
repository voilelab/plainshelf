import { computed, ref } from 'vue';
// TODO(next phase): route this through BookshelfProvider once the dashboard's
// data needs (char_count, per-runtime reading history) are designed into the
// provider interface for mobile/Wails too. For this skeleton we call the API
// layer directly so char_count reaches the page without touching
// providers/bookshelfProvider.ts (out of scope for this task).
import { listBooks } from '../../../api/books';
import type { Book } from '../../../types/book';

export interface StarDistribution {
  1: number;
  2: number;
  3: number;
  4: number;
  5: number;
}

function emptyStarDistribution(): StarDistribution {
  return { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 };
}

function isSameMonth(iso: string | undefined, reference: Date): boolean {
  if (!iso) {
    return false;
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return false;
  }
  return date.getFullYear() === reference.getFullYear() && date.getMonth() === reference.getMonth();
}

export function useDashboardData() {
  const books = ref<Book[]>([]);
  const loading = ref(false);
  const error = ref('');

  const totalBooks = computed(() => books.value.length);

  const addedThisMonth = computed(() => {
    const now = new Date();
    return books.value.filter((book) => isSameMonth(book.created_at, now)).length;
  });

  const starDistribution = computed<StarDistribution>(() => {
    const dist = emptyStarDistribution();
    for (const book of books.value) {
      const star = book.star;
      if (typeof star === 'number' && star >= 1 && star <= 5) {
        const rounded = Math.round(star) as 1 | 2 | 3 | 4 | 5;
        dist[rounded] += 1;
      }
    }
    return dist;
  });

  const starAvg = computed<number | null>(() => {
    const rated = books.value.filter((book) => typeof book.star === 'number' && book.star > 0);
    if (rated.length === 0) {
      return null;
    }
    const sum = rated.reduce((acc, book) => acc + (book.star ?? 0), 0);
    return sum / rated.length;
  });

  const totalChars = computed<number | null>(() => {
    const withCount = books.value.filter((book) => typeof book.char_count === 'number');
    if (withCount.length === 0) {
      return null;
    }
    return withCount.reduce((acc, book) => acc + (book.char_count ?? 0), 0);
  });

  const tagCounts = computed<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    for (const book of books.value) {
      for (const tag of book.tags ?? []) {
        const trimmed = tag.trim();
        if (!trimmed) {
          continue;
        }
        counts[trimmed] = (counts[trimmed] ?? 0) + 1;
      }
    }
    return counts;
  });

  async function fetchDashboardData(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      const data = await listBooks(1, Number.MAX_SAFE_INTEGER, { includeCharCount: true });
      books.value = data.items;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load dashboard data';
    } finally {
      loading.value = false;
    }
  }

  return {
    books,
    loading,
    error,
    totalBooks,
    addedThisMonth,
    starAvg,
    starDistribution,
    totalChars,
    tagCounts,
    fetchDashboardData
  };
}
