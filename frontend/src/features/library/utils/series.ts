import type { Book } from '@/types/book';

interface SeriesSummary {
  name: string;
  count: number;
}

const collator = new Intl.Collator('zh-Hant', { numeric: true, sensitivity: 'base' });

export function summarizeSeries(books: readonly Book[]): SeriesSummary[] {
  const counts = new Map<string, number>();
  for (const book of books) {
    const name = book.series?.name.trim();
    if (name) {
      counts.set(name, (counts.get(name) ?? 0) + 1);
    }
  }
  return [...counts]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => collator.compare(a.name, b.name));
}

export function orderForNumbering<T extends Pick<Book, 'title'>>(books: readonly T[]): T[] {
  return [...books].sort((a, b) => collator.compare(a.title, b.title));
}

export function booksInSeries(books: readonly Book[], name: string): Book[] {
  const wanted = name.trim();
  return books
    .filter((book) => book.series?.name.trim() === wanted)
    .sort((a, b) => (a.series?.index ?? 0) - (b.series?.index ?? 0) || collator.compare(a.title, b.title));
}
