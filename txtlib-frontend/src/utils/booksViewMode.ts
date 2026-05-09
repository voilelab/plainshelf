export type BooksViewMode = 'list' | 'card' | 'title';

export const BOOKS_VIEW_MODE_STORAGE_KEY = 'txtlib.books.viewMode';

const BOOKS_VIEW_MODES: BooksViewMode[] = ['list', 'card', 'title'];

export function isBooksViewMode(value: string | null | undefined): value is BooksViewMode {
  return value !== null && value !== undefined && BOOKS_VIEW_MODES.includes(value as BooksViewMode);
}

function resolveStorageKey(key?: string): string {
  return key && key.trim().length > 0 ? key : BOOKS_VIEW_MODE_STORAGE_KEY;
}

export function getStoredBooksViewMode(storageKey?: string): BooksViewMode {
  if (typeof window === 'undefined') {
    return 'list';
  }

  const value = window.localStorage.getItem(resolveStorageKey(storageKey));
  return isBooksViewMode(value) ? value : 'list';
}

export function setStoredBooksViewMode(mode: BooksViewMode, storageKey?: string): void {
  if (typeof window === 'undefined') {
    return;
  }

  window.localStorage.setItem(resolveStorageKey(storageKey), mode);
}