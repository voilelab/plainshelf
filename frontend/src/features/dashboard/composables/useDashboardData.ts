import { computed, getCurrentInstance, onUnmounted, ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { createShelfInitRetry, isShelfInitializing } from '@/composables/shelfInitRetry';
import { getReadingActivityRange } from '@/storage/readingStats';
import { getReadHistoryIDs } from '@/storage/readHistory';
import { getLocalReadingEntries, getLocalReadingEntry } from '@/storage/readingProgress';
import type { ProgressEntry } from '@/storage/readingProgress';
import type { Book } from '@/types/book';
import { t } from '@/i18n';

const READING_ACTIVITY_RANGE_DAYS = 365;
const RECENT_READING_LIMIT = 6;
const RECENTLY_ADDED_LIMIT = 6;

function createdAtValue(book: Book): number {
  const parsed = book.created_at ? Date.parse(book.created_at) : NaN;
  return Number.isNaN(parsed) ? 0 : parsed;
}

export interface RecentReadingItem {
  book: Book;
  percent: number | null;
  lastReadAt: number | null;
}

function estimateReadingPercent(offset: number, charCount: number | undefined): number | null {
  if (typeof charCount !== 'number' || !Number.isFinite(charCount) || charCount <= 0) {
    return null;
  }
  const ratio = (offset / charCount) * 100;
  return Math.min(100, Math.max(0, Math.round(ratio)));
}

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
  const readingProgress = ref<Record<string, ProgressEntry>>({});
  const recentReading = ref<RecentReadingItem[]>([]);
  const shelfInitializing = ref(false);

  const initRetry = createShelfInitRetry();
  let disposed = false;

  const totalBooks = computed(() => books.value.length);

  const inProgress = computed<number>(() => {
    let count = 0;
    for (const book of books.value) {
      const entry = readingProgress.value[book.id];
      if (!entry || entry.offset <= 0) {
        continue;
      }
      const charCount = book.char_count;
      const finished =
        typeof charCount === 'number' &&
        Number.isFinite(charCount) &&
        charCount > 0 &&
        entry.offset >= charCount;
      if (finished) {
        continue;
      }
      count += 1;
    }
    return count;
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

  const recentlyAdded = computed<Book[]>(() =>
    [...books.value]
      .sort((a, b) => createdAtValue(b) - createdAtValue(a))
      .slice(0, RECENTLY_ADDED_LIMIT)
  );

  const startedBookIds = computed<Set<string>>(() => {
    const ids = new Set<string>();
    for (const [id, entry] of Object.entries(readingProgress.value)) {
      if (entry && entry.offset > 0) {
        ids.add(id);
      }
    }
    return ids;
  });

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

  async function loadRecentReading(all: Book[]): Promise<RecentReadingItem[]> {
    const historyIDs = await getReadHistoryIDs();
    if (historyIDs.length === 0) {
      return [];
    }
    const byID = new Map(all.map((book) => [book.id, book]));
    const recentBooks = historyIDs
      .flatMap((id) => {
        const book = byID.get(id);
        return book ? [book] : [];
      })
      .slice(0, RECENT_READING_LIMIT);
    return Promise.all(
      recentBooks.map(async (book) => {
        const entry = await getLocalReadingEntry(book.id);
        return {
          book,
          percent: entry ? estimateReadingPercent(entry.offset, book.char_count) : null,
          lastReadAt: entry?.at ?? null
        };
      })
    );
  }

  async function run(isAutoRetry: boolean): Promise<void> {
    if (isAutoRetry) {
      initRetry.cancel();
    } else {
      initRetry.reset();
      shelfInitializing.value = false;
    }
    loading.value = true;
    error.value = '';
    try {
      const to = new Date();
      const from = new Date(to);
      from.setDate(from.getDate() - READING_ACTIVITY_RANGE_DAYS);

      const [data, activity] = await Promise.all([
        getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER, { includeCharCount: true }),
        getReadingActivityRange(toIsoDate(from), toIsoDate(to))
      ]);
      books.value = data.items;
      readingActivity.value = activity;
      try {
        recentReading.value = await loadRecentReading(data.items);
      } catch {
        recentReading.value = [];
      }
      try {
        readingProgress.value = await getLocalReadingEntries();
      } catch {
        readingProgress.value = {};
      }
      shelfInitializing.value = false;
      initRetry.reset();
    } catch (err) {
      if (disposed) {
        return;
      }
      if (isShelfInitializing(err)) {
        if (initRetry.schedule(() => void run(true), err)) {
          shelfInitializing.value = true;
          return;
        }
        shelfInitializing.value = false;
        error.value = t('dashboard.shelfNotReady');
      } else {
        shelfInitializing.value = false;
        error.value = err instanceof Error ? err.message : t('dashboard.loadFailed');
      }
    } finally {
      if (!initRetry.pending) {
        loading.value = false;
      }
    }
  }

  async function fetchDashboardData(): Promise<void> {
    await run(false);
  }

  if (getCurrentInstance()) {
    onUnmounted(() => {
      disposed = true;
      initRetry.cancel();
    });
  }

  return {
    books,
    loading,
    error,
    recentReading,
    shelfInitializing,
    totalBooks,
    inProgress,
    starAvg,
    starDistribution,
    totalChars,
    tagCounts,
    heatmapData,
    currentStreak,
    recentlyAdded,
    startedBookIds,
    fetchDashboardData
  };
}
