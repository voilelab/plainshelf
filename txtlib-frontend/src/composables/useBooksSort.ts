import { computed, ref, type Ref } from 'vue';
import type { LocationQueryValue } from 'vue-router';
import type { Book } from '../types/book';
import { toSingleQueryValue } from './useBookPagination';

export const SORT_OPTIONS = ['created_at', 'updated_at', 'title'] as const;
export const ORDER_OPTIONS = ['asc', 'desc'] as const;

export type BookSortKey = (typeof SORT_OPTIONS)[number];
export type SortOrder = (typeof ORDER_OPTIONS)[number];

export function toBookSort(value: LocationQueryValue | LocationQueryValue[] | undefined): BookSortKey {
  const raw = toSingleQueryValue(value);
  return raw && SORT_OPTIONS.includes(raw as BookSortKey) ? (raw as BookSortKey) : 'updated_at';
}

export function toSortOrder(value: LocationQueryValue | LocationQueryValue[] | undefined): SortOrder {
  const raw = toSingleQueryValue(value);
  return raw && ORDER_OPTIONS.includes(raw as SortOrder) ? (raw as SortOrder) : 'desc';
}

export function toTimestampValue(value: string | undefined): number {
  if (!value) {
    return 0;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function useBooksSort(
  books: Ref<Book[]>,
  initialSortQuery: LocationQueryValue | LocationQueryValue[] | undefined,
  initialOrderQuery: LocationQueryValue | LocationQueryValue[] | undefined
) {
  const sortBy = ref<BookSortKey>(toBookSort(initialSortQuery));
  const sortOrder = ref<SortOrder>(toSortOrder(initialOrderQuery));

  const sortedBooks = computed<Book[]>(() => {
    return [...books.value].sort((a, b) => {
      if (sortBy.value === 'title') {
        const result = a.title.localeCompare(b.title, 'zh-Hant', {
          numeric: true,
          sensitivity: 'base'
        });
        return sortOrder.value === 'asc' ? result : -result;
      }

      const aValue = toTimestampValue(
        sortBy.value === 'created_at' ? a.created_at : a.updated_at
      );
      const bValue = toTimestampValue(
        sortBy.value === 'created_at' ? b.created_at : b.updated_at
      );
      const result = aValue - bValue;
      return sortOrder.value === 'asc' ? result : -result;
    });
  });

  function setSortBy(nextSort: BookSortKey): void {
    sortBy.value = nextSort;
  }

  function setSortOrder(nextOrder: SortOrder): void {
    sortOrder.value = nextOrder;
  }

  function toggleOrder(): void {
    setSortOrder(sortOrder.value === 'asc' ? 'desc' : 'asc');
  }

  return {
    SORT_OPTIONS,
    ORDER_OPTIONS,
    sortBy,
    sortOrder,
    sortedBooks,
    toBookSort,
    toSortOrder,
    setSortBy,
    setSortOrder,
    toggleOrder
  };
}
