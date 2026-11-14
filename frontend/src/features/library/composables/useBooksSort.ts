import { computed, type Ref } from 'vue';
import type { Book } from '@/types/book';
import { getPreference, setPreference } from '@/storage/preferences';

export const SORT_OPTIONS = ['created_at', 'updated_at', 'title'] as const;
const ORDER_OPTIONS = ['asc', 'desc'] as const;

export type BookSortKey = (typeof SORT_OPTIONS)[number];
export type SortOrder = (typeof ORDER_OPTIONS)[number];

const SORT_STORAGE_KEY = 'plainshelf.books.sort';
const ORDER_STORAGE_KEY = 'plainshelf.books.order';

export function isBookSortKey(value: string | null | undefined): value is BookSortKey {
  return (SORT_OPTIONS as readonly (string | null | undefined)[]).includes(value);
}

export function isSortOrder(value: string | null | undefined): value is SortOrder {
  return (ORDER_OPTIONS as readonly (string | null | undefined)[]).includes(value);
}

export function getStoredBookSort(): BookSortKey {
  const value = getPreference(SORT_STORAGE_KEY);
  return isBookSortKey(value) ? value : 'updated_at';
}

export function getStoredSortOrder(): SortOrder {
  const value = getPreference(ORDER_STORAGE_KEY);
  return isSortOrder(value) ? value : 'desc';
}

export function storeBooksSort(sort: BookSortKey, order: SortOrder): void {
  setPreference(SORT_STORAGE_KEY, sort);
  setPreference(ORDER_STORAGE_KEY, order);
}

function toTimestampValue(value: string | undefined): number {
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
