import { ref } from 'vue';
import type { LocationQueryValue } from 'vue-router';

const PAGE_SIZE_STORAGE_KEY = 'plainshelf.books.pageSize';
export const PAGE_SIZE_OPTIONS: number[] = [10, 20, 50, 100, 200];
const DEFAULT_PAGE_SIZE = 50;

function loadPageSize(): number {
  if (typeof window === 'undefined') {
    return DEFAULT_PAGE_SIZE;
  }
  const raw = window.localStorage.getItem(PAGE_SIZE_STORAGE_KEY);
  if (!raw) {
    return DEFAULT_PAGE_SIZE;
  }
  const parsed = Number(raw);
  return (PAGE_SIZE_OPTIONS as readonly number[]).includes(parsed) ? parsed : DEFAULT_PAGE_SIZE;
}

// Module-level singleton: shared across all components that call useBookPagination()
const pageSize = ref<number>(loadPageSize());

export function setPageSize(newSize: number): void {
  if (!(PAGE_SIZE_OPTIONS as readonly number[]).includes(newSize)) {
    return;
  }
  pageSize.value = newSize;
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(PAGE_SIZE_STORAGE_KEY, String(newSize));
  }
}

export function toSingleQueryValue(
  value: LocationQueryValue | LocationQueryValue[] | undefined
): string | undefined {
  const raw = Array.isArray(value) ? value[0] : value;
  return typeof raw === 'string' ? raw : undefined;
}

export function toPage(
  value: LocationQueryValue | LocationQueryValue[] | undefined
): number {
  const raw = toSingleQueryValue(value);
  if (!raw) {
    return 1;
  }
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed < 1) {
    return 1;
  }
  return parsed;
}

export function useBookPagination() {
  return {
    pageSize,
    setPageSize,
    PAGE_SIZE_OPTIONS,
    toSingleQueryValue,
    toPage
  };
}
