import { computed, watch, type ComputedRef, type Ref } from 'vue';
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router';
import { useBookPagination, toPage, toSingleQueryValue } from '@/composables/useBookPagination';
import type { Book } from '@/types/book';

/** Page count for `total` items. Never below 1, so an empty list still has page 1. */
export function countPages(total: number, pageSize: number): number {
  return Math.max(1, Math.ceil(total / pageSize));
}

/** The slice of `items` shown on `page` (1-based). */
export function pageSlice<T>(items: readonly T[], page: number, pageSize: number): T[] {
  const start = (page - 1) * pageSize;
  return items.slice(start, start + pageSize);
}

export interface UseBookCollectionRouteOptions<T> {
  /**
   * The complete list to paginate, with the caller's own filtering already
   * applied. The page-sized slice is cut here, not by the caller.
   */
  items: Ref<T[]> | ComputedRef<T[]>;
  /**
   * Builds the route query for a page change. Injected rather than shared: each
   * page owns extra query keys (`maxChars`, …) and the key order they produce is
   * part of the URL those pages already emit.
   */
  buildQuery: (page: number) => LocationQueryRaw;
  /**
   * Gates the clamp watcher below. Omit it when the list is already in hand on
   * setup; pass a "first load finished" flag when it is fetched asynchronously,
   * otherwise the still-empty list clamps a deep-linked `?page=3` back to 1
   * before the response arrives.
   */
  clampEnabled?: Ref<boolean> | ComputedRef<boolean>;
}

/**
 * Route-backed pagination for the list pages built on BookCollectionPage: the
 * current page lives in the URL, the page size in the shared store, and the
 * visible slice follows from both.
 *
 * LibraryPage deliberately does not use this — its page changes go through
 * useBooksRouteQuery, and its clamp watcher carries extra guards for search and
 * initial load. It shares only `countPages`/`pageSlice` above.
 */
export function useBookCollectionRoute<T = Book>(options: UseBookCollectionRouteOptions<T>) {
  const route = useRoute();
  const router = useRouter();
  const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();

  const page = computed(() => toPage(route.query.page));
  const totalPages = computed(() => countPages(options.items.value.length, pageSize.value));
  const visibleBooks = computed(() => pageSlice(options.items.value, page.value, pageSize.value));

  function onPageChange(nextPage: number): void {
    if (nextPage === page.value) {
      return;
    }

    void router.push({
      path: route.path,
      query: options.buildQuery(nextPage)
    });
  }

  function onPageSizeChange(newSize: number): void {
    setPageSize(newSize);
    void router.push({
      path: route.path,
      query: options.buildQuery(1)
    });
  }

  // Keep the URL's page within range as the list shrinks (filtering, deleting,
  // clearing history). Runs immediately so a hand-typed page lands in range.
  watch(
    [page, totalPages, () => options.clampEnabled?.value ?? true],
    ([currentPage, maxPage, enabled]) => {
      if (!enabled) {
        return;
      }

      const normalizedPage = Math.min(currentPage, maxPage);
      if (toSingleQueryValue(route.query.page) === String(normalizedPage)) {
        return;
      }

      void router.replace({
        path: route.path,
        query: options.buildQuery(normalizedPage)
      });
    },
    { immediate: true }
  );

  return {
    page,
    pageSize,
    totalPages,
    visibleBooks,
    onPageChange,
    onPageSizeChange,
    PAGE_SIZE_OPTIONS
  };
}
