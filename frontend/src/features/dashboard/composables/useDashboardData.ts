import { computed, getCurrentInstance, onUnmounted, ref } from 'vue';
// Books come through the provider so the dashboard works on every backend —
// including a pCloud connection, which has no HTTP API to call. char_count is
// opt-in on the provider interface, which is what this page needs from it.
// Reading activity does not go through the provider by design: it is
// device-local and never involves the server.
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import { getReadingActivityRange } from '@/storage/readingStats';
import { getReadHistoryIDs } from '@/storage/readHistory';
import { getLocalReadingEntries, getLocalReadingEntry } from '@/storage/readingProgress';
import type { ProgressEntry } from '@/storage/readingProgress';
import type { Book } from '@/types/book';
import { t } from '@/i18n';

const READING_ACTIVITY_RANGE_DAYS = 365;
const SHELF_INIT_RETRY_DELAY_MS = 3000;
const SHELF_INIT_MAX_AUTO_RETRIES = 10; // ~30s of auto-retry before showing an error
const RECENT_READING_LIMIT = 6;
const RECENTLY_ADDED_LIMIT = 6;

// created_at is an ISO timestamp string, but optional and occasionally legacy;
// a missing or unparseable value sorts as 0 (oldest) rather than throwing, the
// same forgiving contract the library's sort uses.
function createdAtValue(book: Book): number {
  const parsed = book.created_at ? Date.parse(book.created_at) : NaN;
  return Number.isNaN(parsed) ? 0 : parsed;
}

export interface RecentReadingItem {
  book: Book;
  /** 0–100, or null when there is no progress entry or no known char_count. */
  percent: number | null;
  /** Epoch ms of the last read, or null when unknown (e.g. the mobile shell). */
  lastReadAt: number | null;
}

/**
 * The home-page progress percentage is `offset / char_count`, a lightweight
 * approximation that avoids reloading each book's content the way the reader does.
 * It is null — so the card renders without a progress bar — whenever char_count is
 * missing or non-positive, so a missing count never shows as NaN or a false 0%.
 *
 * The two values use different units: `offset` is a UTF-16 index (what the reader
 * advances), while the server's `char_count` is a rune count (len([]rune)). They
 * agree for BMP text, including most CJK, but a supplementary-plane character — an
 * emoji, a CJK Extension B glyph — adds two to `offset` and one to `char_count`, so
 * an emoji-heavy book overshoots. The clamp to [0, 100] keeps the worst case an
 * early 100% rather than NaN or an out-of-range bar. A unit-exact figure would need
 * each book's loaded content, which this list avoids by design; the reader itself
 * still shows the precise percentage.
 */
export function computeReadingPercent(offset: number, charCount: number | undefined): number | null {
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

  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let initRetryCount = 0;
  let disposed = false;

  function clearRetry(): void {
    if (retryTimer !== null) {
      clearTimeout(retryTimer);
      retryTimer = null;
    }
  }

  const totalBooks = computed(() => books.value.length);

  // Books that have been opened (a progress entry with a positive offset) but are
  // not finished yet. Reuses the same device-local reading-progress store the
  // recent-reading list reads from. A zero-offset entry is a reset tombstone, not
  // progress. Completion is tested unrounded — the saved offset reaching the
  // book's length — rather than through computeReadingPercent, whose rounding
  // would report 199/200 as 100% and drop a book that is still short of the end.
  // A positive offset with an unknown or invalid char_count has demonstrable
  // progress but no provable end, so it stays counted.
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

  // The six most recently added books, newest first, drawn from the already
  // loaded listing so the home page's "recently added" row costs no extra
  // request. sort() is stable, so books that share (or lack) a created_at keep
  // the listing's own order.
  const recentlyAdded = computed<Book[]>(() =>
    [...books.value]
      .sort((a, b) => createdAtValue(b) - createdAtValue(a))
      .slice(0, RECENTLY_ADDED_LIMIT)
  );

  // Ids of books the reader has actually opened (a progress entry past the
  // start). The random pick prefers books absent from this set so it surfaces
  // something unread; a reset tombstone (offset 0) is not "started". Empty on
  // the mobile shell, whose progress lives outside getLocalReadingEntries, so
  // there the pick simply draws from everything.
  const startedBookIds = computed<Set<string>>(() => {
    const ids = new Set<string>();
    for (const [id, entry] of Object.entries(readingProgress.value)) {
      if (entry && entry.offset > 0) {
        ids.add(id);
      }
    }
    return ids;
  });

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

  // The recent-reading list reuses the books already loaded above (which carry
  // char_count), ordered by the device's read history and capped to the newest
  // few, so it needs no second listing. Progress and last-read time come from the
  // local reading-progress store per book; the mobile shell keeps progress
  // elsewhere, so there getLocalReadingEntry returns null and the card shows
  // without a progress bar.
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
          percent: entry ? computeReadingPercent(entry.offset, book.char_count) : null,
          lastReadAt: entry?.at ?? null
        };
      })
    );
  }

  async function run(isAutoRetry: boolean): Promise<void> {
    clearRetry();
    if (!isAutoRetry) {
      initRetryCount = 0;
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
      // Recent-reading and the in-progress count are convenience state read from
      // device-local history and progress; a failure in either must not blank the
      // dashboard that just loaded, and one failing must not discard the other.
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
      initRetryCount = 0;
    } catch (err) {
      // The page can go away while the request is in flight, and cancelling on
      // unmount only reaches a timer that already exists. Scheduling a retry
      // from here would outlive the page and keep polling for the rest of the
      // budget, alongside whatever a freshly mounted dashboard is requesting.
      if (disposed) {
        return;
      }
      // A shelf still running its initial scan answers 503 for every read. The
      // book listing and the folder tree retry through that, so the dashboard
      // has to as well: otherwise a cold start puts an error in the middle of
      // the home page while the sidebar beside it is quietly recovering.
      if (err instanceof ApiError && err.status === 503) {
        initRetryCount++;
        if (initRetryCount < SHELF_INIT_MAX_AUTO_RETRIES) {
          shelfInitializing.value = true;
          retryTimer = setTimeout(() => void run(true), SHELF_INIT_RETRY_DELAY_MS);
          return;
        }
        shelfInitializing.value = false;
        error.value = t('dashboard.shelfNotReady');
      } else {
        shelfInitializing.value = false;
        error.value = err instanceof Error ? err.message : t('dashboard.loadFailed');
      }
    } finally {
      // A pending retry keeps the page in its loading state rather than
      // flashing an error between attempts.
      if (retryTimer === null) {
        loading.value = false;
      }
    }
  }

  // Takes no arguments on purpose: it is bound straight to a template click
  // handler, which would otherwise pass the event in as the retry flag.
  async function fetchDashboardData(): Promise<void> {
    await run(false);
  }

  // These refs are page-scoped, so neither a pending retry nor a request that
  // is still in flight may outlive the page.
  if (getCurrentInstance()) {
    onUnmounted(() => {
      disposed = true;
      clearRetry();
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
