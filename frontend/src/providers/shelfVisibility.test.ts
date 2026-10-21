import { describe, expect, it } from 'vitest';

import { ShelfVisibility } from './shelfVisibility';
import type { Book } from '@/types/book';

function book(id: string, folders: string[], mark?: Partial<Pick<Book, 'nsfw' | 'nsfw_folder'>>): Book {
  return { id, title: id, authors: [], tags: [], folders, ...mark };
}

describe('ShelfVisibility.allows', () => {
  it.each([
    ['an unmarked book', {}, true],
    ['a book marked in its own book.json', { nsfw: true }, false],
    ['a book in a marked folder', { nsfw_folder: { path: 'Adult' } }, false],
    ['a book that refused its own mark inside a marked folder', { nsfw: false, nsfw_folder: { path: 'Adult' } }, false]
  ])('hides %s with the setting off', (_label, mark, want) => {
    const visibility = new ShelfVisibility({ showNsfw: false });

    expect(visibility.allows(book('a', [], mark))).toBe(want);
  });

  it('allows everything with the setting on', () => {
    const visibility = new ShelfVisibility({ showNsfw: true });

    expect(visibility.allows(book('a', [], { nsfw: true }))).toBe(true);
    expect(visibility.allows(book('a', [], { nsfw_folder: { path: 'Adult' } }))).toBe(true);
  });
});

describe('ShelfVisibility.keepBooks', () => {
  it('drops the marked books, keeping the order of the rest', () => {
    const books = [book('a', []), book('b', [], { nsfw: true }), book('c', [])];

    expect(new ShelfVisibility({ showNsfw: false }).keepBooks(books).map((entry) => entry.id)).toEqual([
      'a',
      'c'
    ]);
  });

  it('returns the listing untouched with the setting on', () => {
    const books = [book('a', []), book('b', [], { nsfw: true })];

    expect(new ShelfVisibility({ showNsfw: true }).keepBooks(books)).toBe(books);
  });
});

describe('ShelfVisibility.filterFolders', () => {
  const isNsfwFolder = (folders: readonly string[]) => folders[0] === 'Adult';

  function filter(folders: string[], books: Book[], showNsfw = false): string[] {
    return new ShelfVisibility({ showNsfw, isNsfwFolder }).filterFolders(folders, books);
  }

  it('drops a marked folder and everything under it', () => {
    expect(filter(['/', 'Adult', 'Adult/2024', 'Fiction'], [])).toEqual(['/', 'Fiction']);
  });

  it('drops a folder holding nothing but hidden books', () => {
    const books = [book('a', ['Fiction']), book('b', ['Doujin'], { nsfw: true })];

    expect(filter(['/', 'Doujin', 'Fiction'], books)).toEqual(['/', 'Fiction']);
  });

  it('keeps a folder holding no book at all, which nobody hid', () => {
    expect(filter(['/', 'Empty'], [book('a', ['Fiction'])])).toEqual(['/', 'Empty']);
  });

  it('keeps an ancestor of a visible book', () => {
    const books = [book('a', ['Fiction'], { nsfw: true }), book('b', ['Fiction', 'Classics'])];

    expect(filter(['/', 'Fiction', 'Fiction/Classics'], books)).toEqual([
      '/',
      'Fiction',
      'Fiction/Classics'
    ]);
  });
});
