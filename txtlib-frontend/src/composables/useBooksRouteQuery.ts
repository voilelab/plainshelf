import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { LocationQueryValue, NavigationFailure } from 'vue-router';
import { toPage, toSingleQueryValue } from './useBookPagination';
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

  const selectedLayer = computed(() => toLayerPath(route.query.layers) ?? toLayerPath(route.query.layer));
  const page = computed(() => toPage(route.query.page));
  const sortBy = computed<BookSortKey>(() => toBookSort(route.query.sort));
  const sortOrder = computed<SortOrder>(() => toSortOrder(route.query.order));
  const searchQuery = computed(() => (toSingleQueryValue(route.query.search) ?? '').trim());
  const isImportModalOpen = computed(() => toSingleQueryValue(route.query.import) === '1');

  function buildBooksQuery(input: BooksQueryInput = {}) {
    const nextQuery = {
      ...route.query
    } as Record<string, LocationQueryValue | LocationQueryValue[]>;

    delete nextQuery.layer;
    delete nextQuery.layers;
    delete nextQuery.page;
    delete nextQuery.search;
    delete nextQuery.sort;
    delete nextQuery.order;

    const layerValue = input.layer !== undefined ? input.layer : selectedLayer.value;
    const normalizedLayer = layerValue?.trim();
    if (normalizedLayer) {
      nextQuery.layers = normalizedLayer;
    }

    const pageValue = input.page !== undefined ? input.page : page.value;
    const normalizedPage = Number.isInteger(pageValue) && pageValue > 0 ? pageValue : 1;
    nextQuery.page = String(normalizedPage);

    const searchValue = input.search !== undefined ? input.search : searchQuery.value;
    const normalizedSearch = searchValue.trim();
    if (normalizedSearch) {
      nextQuery.search = normalizedSearch;
    }

    nextQuery.sort = input.sort ?? sortBy.value;
    nextQuery.order = input.order ?? sortOrder.value;

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
    isImportModalOpen,
    toLayerPath,
    toBookSort,
    toSortOrder,
    pushBooksQuery,
    replaceBooksQuery,
    isBooksQueryNormalized,
    openImportModalQuery,
    closeImportModalQuery
  };
}
