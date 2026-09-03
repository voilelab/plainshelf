import { getCurrentInstance, onUnmounted, ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import { t } from '@/i18n';

// A lazily loaded book ID -> char_count map for the pages that filter on it.
//
// char_count makes the backend open every book's current source, so it is kept
// off the shared useBookStore listing and only requested once a char-count
// filter is actually in use. Only the counts are kept here: the book list
// itself stays on the shared store, so enabling the filter never forks the
// data every other action on the page reads from.
//
// Each call returns its own refs, so mounting a page that filters on character
// counts never changes what other pages fetch.
export function useCharCountIndex() {
  const counts = ref<Map<string, number | undefined>>(new Map());
  const ready = ref(false);
  const loading = ref(false);
  const error = ref('');

  const initRetry = createShelfInitRetry();
  // Bumped by every load()/invalidate() so a response from a superseded
  // request cannot overwrite fresher counts.
  let generation = 0;

  /** Drops the cached counts and cancels any in-flight load. */
  function invalidate(): void {
    generation++;
    initRetry.cancel();
    counts.value = new Map();
    ready.value = false;
    loading.value = false;
    error.value = '';
  }

  /**
   * Refetches the counts without dropping the cached ones first, so a caller
   * that renders while `ready` can keep showing the previous counts instead of
   * collapsing into a loading state for the duration of the request.
   */
  async function refresh(): Promise<void> {
    generation++;
    await run(generation, false);
  }

  async function run(token: number, isAutoRetry: boolean): Promise<void> {
    if (isAutoRetry) {
      initRetry.cancel();
    } else {
      initRetry.reset();
    }
    loading.value = true;
    error.value = '';
    try {
      const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER, {
        includeCharCount: true
      });
      if (token !== generation) {
        return;
      }
      counts.value = new Map(data.items.map((book) => [book.id, book.char_count]));
      ready.value = true;
      initRetry.reset();
    } catch (err) {
      if (token !== generation) {
        return;
      }
      // Wait out the initial scan rather than stranding the filter on an error
      // the shelf resolves on its own. See `shelfInitRetry`.
      if (isShelfInitializing(err)) {
        if (initRetry.schedule(() => void run(token, true), err)) {
          return;
        }
        error.value = t('library.shelfNotReady');
      } else {
        error.value = err instanceof ApiError && err.isTimeout
          ? t('library.requestTimeout')
          : err instanceof Error ? err.message : t('library.loadFailed');
      }
    } finally {
      if (token === generation && !initRetry.pending) {
        loading.value = false;
      }
    }
  }

  /** Loads the counts unless they are already loaded or a load is in flight. */
  async function load(): Promise<void> {
    if (ready.value || loading.value) {
      return;
    }
    await refresh();
  }

  // These refs are page-scoped, so a pending retry must not outlive the page.
  if (getCurrentInstance()) {
    onUnmounted(initRetry.cancel);
  }

  return {
    counts,
    ready,
    loading,
    error,
    load,
    refresh,
    invalidate
  };
}
