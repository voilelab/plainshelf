import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { LocationQueryValue, NavigationFailure } from 'vue-router';
import { toPage, toSingleQueryValue } from '@/composables/useBookPagination';
import type { CharCountRange } from '@/utils/charCountFilter';
import {
  BOOK_FILTERS,
  charCountFilter,
  layersFilter,
  searchFilter
} from '@/utils/bookFilters/registry';
import {
  ORDER_OPTIONS,
  SORT_OPTIONS,
  type BookSortKey,
  type SortOrder
} from './useBooksSort';

export type BooksQueryInput = {
  layer?: string;
  page?: number;
  search?: string;
  sort?: BookSortKey;
  order?: SortOrder;
  /**
   * Character-count bounds. Omitting the key carries the current bounds
   * through; passing a range replaces both, so `{ charCount: {} }` clears the
   * filter rather than leaving it untouched.
   */
  charCount?: CharCountRange;
};

type RouterNavigationResult = Promise<void | NavigationFailure | undefined>;

function isBookSortKey(value: string): value is BookSortKey {
  return (SORT_OPTIONS as readonly string[]).includes(value);
}

function isSortOrder(value: string): value is SortOrder {
  return (ORDER_OPTIONS as readonly string[]).includes(value);
}

export function toLayerPath(value: LocationQueryValue | LocationQueryValue[] | undefined): string | undefined {
  const raw = toSingleQueryValue(value);
  if (!raw) {
    return undefined;
  }

  const normalized = raw.trim();
  return normalized.length > 0 ? normalized : undefined;
}

export function toBookSort(value: LocationQueryValue | LocationQueryValue[] | undefined): BookSortKey {
  const raw = toSingleQueryValue(value);
  if (!raw) {
    return 'updated_at';
  }
  return isBookSortKey(raw) ? raw : 'updated_at';
}

export function toSortOrder(value: LocationQueryValue | LocationQueryValue[] | undefined): SortOrder {
  const raw = toSingleQueryValue(value);
  if (!raw) {
    return 'desc';
  }
  return isSortOrder(raw) ? raw : 'desc';
}

function queryValueEquals(
  left: LocationQueryValue | LocationQueryValue[] | undefined,
  right: LocationQueryValue | LocationQueryValue[] | undefined
): boolean {
  const leftArray = Array.isArray(left) ? left : undefined;
  const rightArray = Array.isArray(right) ? right : undefined;

  if (leftArray || rightArray) {
    if (!leftArray || !rightArray || leftArray.length !== rightArray.length) {
      return false;
    }

    return leftArray.every((value, index) => value === rightArray[index]);
  }

  return left === right;
}

function queryObjectEquals(
  left: Record<string, LocationQueryValue | LocationQueryValue[]>,
  right: Record<string, LocationQueryValue | LocationQueryValue[]>
): boolean {
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);

  if (leftKeys.length !== rightKeys.length) {
    return false;
  }

  return leftKeys.every((key) => queryValueEquals(left[key], right[key]));
}

export function useBooksRouteQuery() {
  const route = useRoute();
  const router = useRouter();

  // The reactive reads share the registry's parsers, so a filter's URL shape is
  // defined once and both the query builder and these computeds see it.
  const selectedLayer = computed(() => layersFilter.parse(route.query));
  const page = computed(() => toPage(route.query.page));
  const sortBy = computed<BookSortKey>(() => toBookSort(route.query.sort));
  const sortOrder = computed<SortOrder>(() => toSortOrder(route.query.order));
  const searchQuery = computed(() => searchFilter.parse(route.query));
  const charCountRange = computed<CharCountRange>(() => charCountFilter.parse(route.query));
  const isImportModalOpen = computed(() => toSingleQueryValue(route.query.import) === '1');

  function buildBooksQuery(
    input: BooksQueryInput = {},
    panelOverrides?: Record<string, unknown>
  ) {
    const nextQuery = {
      ...route.query
    } as Record<string, LocationQueryValue | LocationQueryValue[]>;

    // Clear every filter-owned key up front so a condition that is now inactive
    // leaves nothing behind, then let each filter write itself back below.
    for (const filter of BOOK_FILTERS) {
      for (const key of filter.queryKeys) {
        delete nextQuery[key];
      }
    }

    // page/sort/order are pagination and ordering, not filters: always present.
    const pageValue = input.page !== undefined ? input.page : page.value;
    const normalizedPage = Number.isInteger(pageValue) && pageValue > 0 ? pageValue : 1;
    nextQuery.page = String(normalizedPage);
    nextQuery.sort = input.sort ?? sortBy.value;
    nextQuery.order = input.order ?? sortOrder.value;

    // Each filter re-serializes either its explicit input override or what the
    // current query already carries. The typed override keys mirror
    // BooksQueryInput; `panelOverrides` is the generic path any panel condition
    // uses, where a key being *present* — even with a cleared value — overrides,
    // so removing a chip is `{ author: undefined }` rather than a sentinel.
    const overrides: Record<string, unknown> = {
      [searchFilter.key]: input.search,
      [layersFilter.key]: input.layer,
      [charCountFilter.key]: input.charCount
    };
    for (const filter of BOOK_FILTERS) {
      let value: unknown;
      if (panelOverrides && filter.key in panelOverrides) {
        value = panelOverrides[filter.key];
      } else {
        const override = overrides[filter.key];
        value = override !== undefined ? override : filter.parse(route.query);
      }
      if (filter.isActive(value)) {
        Object.assign(nextQuery, filter.serialize(value));
      }
    }

    return nextQuery;
  }

  function pushBooksQuery(input: BooksQueryInput = {}): RouterNavigationResult {
    return router.push({
      path: '/books',
      query: buildBooksQuery(input)
    });
  }

  function replaceBooksQuery(input: BooksQueryInput = {}): RouterNavigationResult {
    return router.replace({
      path: '/books',
      query: buildBooksQuery(input)
    });
  }

  /**
   * Sets or clears any panel condition by key (the generic path for the filter
   * panel, its chips, and "clear all"). Each override value is a filter's own
   * value type — an inactive value (e.g. `undefined`, `{}`) clears it. Always
   * resets to page 1, since the visible set changes, and leaves search, layer,
   * sort, and order untouched.
   */
  function applyPanelFilters(overrides: Record<string, unknown>): RouterNavigationResult {
    return router.replace({
      path: '/books',
      query: buildBooksQuery({ page: 1 }, overrides)
    });
  }

  function isBooksQueryNormalized(input: BooksQueryInput = {}): boolean {
    if (toSingleQueryValue(route.query.layer) !== undefined) {
      return false;
    }

    const expected = buildBooksQuery(input);
    const current = route.query as Record<string, LocationQueryValue | LocationQueryValue[]>;
    return queryObjectEquals(current, expected);
  }

  function buildImportQuery(open: boolean) {
    const nextQuery = {
      ...route.query
    } as Record<string, LocationQueryValue | LocationQueryValue[]>;

    if (open) {
      nextQuery.import = '1';
    } else {
      delete nextQuery.import;
    }

    return nextQuery;
  }

  function openImportModalQuery(): RouterNavigationResult {
    return router.push({
      path: '/books',
      query: buildImportQuery(true)
    });
  }

  function closeImportModalQuery(): RouterNavigationResult {
    return router.replace({
      path: '/books',
      query: buildImportQuery(false)
    });
  }

  return {
    selectedLayer,
    page,
    sortBy,
    sortOrder,
    searchQuery,
    charCountRange,
    isImportModalOpen,
    toLayerPath,
    toBookSort,
    toSortOrder,
    pushBooksQuery,
    replaceBooksQuery,
    applyPanelFilters,
    isBooksQueryNormalized,
    openImportModalQuery,
    closeImportModalQuery
  };
}
