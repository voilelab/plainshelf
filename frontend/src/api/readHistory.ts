import { getBook, listBooks, mockBooks } from './books';
import { fetchJson, isMockApiMode } from './client';
import type { Book } from '../types/book';

let mockReadHistory = mockBooks.slice(0, 3).map((book) => book.id);

function delay<T>(value: T, ms = 240): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), ms);
  });
}

export async function getReadHistoryIDs(): Promise<string[]> {
  if (isMockApiMode()) {
    return delay([...mockReadHistory]);
  }

  return await fetchJson<string[]>('/api/read_history');
}

export async function addReadHistory(bookID: string): Promise<void> {
  const trimmed = bookID.trim();
  if (!trimmed) {
    return;
  }

  if (isMockApiMode()) {
    mockReadHistory = [trimmed, ...mockReadHistory.filter((id) => id !== trimmed)];
    await delay(undefined);
    return;
  }

  await fetchJson<void>(`/api/read_history?book_id=${encodeURIComponent(trimmed)}`, {
    method: 'POST'
  });
}

export async function clearReadHistory(): Promise<void> {
  if (isMockApiMode()) {
    mockReadHistory = [];
    await delay(undefined);
    return;
  }

  await fetchJson<void>('/api/read_history', {
    method: 'DELETE'
  });
}

export async function listReadHistoryBooks(): Promise<Book[]> {
  const historyIDs = await getReadHistoryIDs();
  if (historyIDs.length === 0) {
    return [];
  }

  if (isMockApiMode()) {
    const books = await Promise.allSettled(historyIDs.map((id) => getBook(id)));
    return books
      .filter((result): result is PromiseFulfilledResult<Book> => result.status === 'fulfilled')
      .map((result) => result.value);
  }

  const allBooks = await listBooks(1, Number.MAX_SAFE_INTEGER);
  const bookByID = new Map(allBooks.items.map((book) => [book.id, book]));
  return historyIDs.flatMap((id) => {
    const book = bookByID.get(id);
    return book ? [book] : [];
  });
}
