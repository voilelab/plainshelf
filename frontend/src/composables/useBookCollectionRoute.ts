import { computed, watch, type ComputedRef, type Ref } from 'vue';
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router';
import { useBookPagination, toPage, toSingleQueryValue } from '@/composables/useBookPagination';
import type { Book } from '@/types/book';

export function countPages(total: number, pageSize: number): number {
  return Math.max(1, Math.ceil(total / pageSize));
}

export function pageSlice<T>(items: readonly T[], page: number, pageSize: number): T[] {
  const start = (page - 1) * pageSize;
  return items.slice(start, start + pageSize);
}

interface UseBookCollectionRouteOptions<T> {
  items: Ref<T[]> | ComputedRef<T[]>;
  buildQuery: (page: number) => LocationQueryRaw;
  clampEnabled?: Ref<boolean> | ComputedRef<boolean>;
}

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

  watch(
    [page, totalPages, () => options.clampEnabled?.value ?? true],
    ([currentPage, maxPage, enabled]) => {
      if (!enabled) {
        return;
      }

      const normalizedPage = Math.min(currentPage, maxPage);
      const queryPage = toSingleQueryValue(route.query.page);
      if (queryPage === String(normalizedPage) || (queryPage === undefined && normalizedPage === 1)) {
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
