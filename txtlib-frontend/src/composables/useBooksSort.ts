import { computed, type Ref } from 'vue';
import type { Book } from '../types/book';

export const SORT_OPTIONS = ['created_at', 'updated_at', 'title'] as const;
export const ORDER_OPTIONS = ['asc', 'desc'] as const;

export type BookSortKey = (typeof SORT_OPTIONS)[number];
export type SortOrder = (typeof ORDER_OPTIONS)[number];

export function toTimestampValue(value: string | undefined): number {
  if (!value) {
    return 0;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function useBooksSort(
  books: Ref<Book[]>,
  sortBy: Readonly<Ref<BookSortKey>>,
  sortOrder: Readonly<Ref<SortOrder>>
) {
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

  return {
    SORT_OPTIONS,
    ORDER_OPTIONS,
    sortedBooks
  };
}
