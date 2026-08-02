import { computed, ref } from 'vue';
// TODO(next phase): route listBooks through BookshelfProvider once the
// dashboard's char_count need is designed into the provider interface for
// mobile/Wails too. For this skeleton we call the API layer directly so
// char_count reaches the page without touching providers/bookshelfProvider.ts
// (out of scope for this task). Reading activity does not go through the
// provider by design: it is device-local and never involves the server.
import { listBooks } from '@/api/books';
import { getReadingActivityRange } from '@/storage/readingStats';
import type { Book } from '@/types/book';

const READING_ACTIVITY_RANGE_DAYS = 365;

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

function toIsoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function useDashboardData() {
  const books = ref<Book[]>([]);
  const loading = ref(false);
  const error = ref('');
  const readingActivity = ref<Record<string, number>>({});

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

  const heatmapData = computed<Record<string, number>>(() => readingActivity.value);

  // Consecutive days (ending today) with total_seconds > 0. If today has no
  // recorded reading yet (the common case — the day isn't over), the streak
  // is counted starting from yesterday instead, so an ongoing streak doesn't
  // appear to reset to 0 first thing in the morning.
  const currentStreak = computed<number>(() => {
    const cursor = new Date();
    cursor.setHours(0, 0, 0, 0);

    if ((readingActivity.value[toIsoDate(cursor)] ?? 0) <= 0) {
      cursor.setDate(cursor.getDate() - 1);
    }

    let streak = 0;
    for (let i = 0; i <= READING_ACTIVITY_RANGE_DAYS; i += 1) {
      const seconds = readingActivity.value[toIsoDate(cursor)] ?? 0;
      if (seconds <= 0) {
        break;
      }
      streak += 1;
      cursor.setDate(cursor.getDate() - 1);
    }
    return streak;
  });

  async function fetchDashboardData(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      const to = new Date();
      const from = new Date(to);
      from.setDate(from.getDate() - READING_ACTIVITY_RANGE_DAYS);

      const [data, activity] = await Promise.all([
        listBooks(1, Number.MAX_SAFE_INTEGER, { includeCharCount: true }),
        getReadingActivityRange(toIsoDate(from), toIsoDate(to))
      ]);
      books.value = data.items;
      readingActivity.value = activity;
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
    heatmapData,
    currentStreak,
    fetchDashboardData
  };
}
