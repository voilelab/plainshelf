import { getReadHistoryIDs } from '@/storage/readHistory';
import type { Book, PaginatedBooks } from '@/types/book';

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
