import { getReadHistoryIDs } from '@/storage/readHistory';
import type { Book, PaginatedBooks } from '@/types/book';

/**
 * Hydrates the locally stored reading-history ids into books, in history order.
 *
 * The ids come from the device; the books come from whichever listing the
 * calling provider uses, so the mobile provider automatically resolves them
 * from its offline cache when the server is unreachable. Ids with no matching
 * book (deleted since it was read) are dropped.
 */
export async function collectReadHistoryBooks(
  listBooks: (page: number, pageSize: number) => Promise<PaginatedBooks>
): Promise<Book[]> {
  const historyIDs = await getReadHistoryIDs();
  if (historyIDs.length === 0) {
    return [];
  }

  const allBooks = await listBooks(1, Number.MAX_SAFE_INTEGER);
  const bookByID = new Map(allBooks.items.map((book) => [book.id, book]));
  return historyIDs.flatMap((id) => {
    const book = bookByID.get(id);
    return book ? [book] : [];
  });
}
