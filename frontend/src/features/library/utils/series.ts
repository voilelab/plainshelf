import type { Book } from '@/types/book';

interface SeriesSummary {
  name: string;
  count: number;
  coverBook: Book;
}

const collator = new Intl.Collator('zh-Hant', { numeric: true, sensitivity: 'base' });

function compareSeriesOrder(a: Book, b: Book): number {
  return (a.series?.index ?? 0) - (b.series?.index ?? 0) || collator.compare(a.title, b.title);
}

function pickCoverBook(ordered: readonly Book[]): Book {
  return ordered.find((book) => book.cover_url) ?? ordered[0];
}

export function summarizeSeries(books: readonly Book[]): SeriesSummary[] {
  const groups = new Map<string, Book[]>();
  for (const book of books) {
    const name = book.series?.name.trim();
    if (name) {
      const group = groups.get(name);
      if (group) {
        group.push(book);
      } else {
        groups.set(name, [book]);
      }
    }
  }
  return [...groups]
    .map(([name, members]) => ({
      name,
      count: members.length,
      coverBook: pickCoverBook(members.sort(compareSeriesOrder))
    }))
    .sort((a, b) => collator.compare(a.name, b.name));
}

export function orderForNumbering<T extends Pick<Book, 'title'>>(books: readonly T[]): T[] {
  return [...books].sort((a, b) => collator.compare(a.title, b.title));
}

export function booksInSeries(books: readonly Book[], name: string): Book[] {
  const wanted = name.trim();
  return books.filter((book) => book.series?.name.trim() === wanted).sort(compareSeriesOrder);
}
