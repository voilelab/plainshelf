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
    // The two halves add: clearing the book's own mark does not take it out of
    // a marked folder, the same sum isBookNsfw and Shelf.IsBookNSFW compute.
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
  // Mirrors the shelf rules: a rule marks its own folder and everything below.
  const isNsfwFolder = (folders: readonly string[]) => folders[0] === 'Adult';

  function filter(folders: string[], books: Book[], showNsfw = false): string[] {
    return new ShelfVisibility({ showNsfw, isNsfwFolder }).filterFolders(folders, books);
  }

  it('drops a marked folder and everything under it', () => {
    expect(filter(['/', 'Adult', 'Adult/2024', 'Fiction'], [])).toEqual(['/', 'Fiction']);
  });

  // A folder left empty by the filter is the disclosure the filter exists to
  // prevent: its name usually says what it held.
  it('drops a folder holding nothing but hidden books', () => {
    const books = [book('a', ['Fiction']), book('b', ['Doujin'], { nsfw: true })];

    expect(filter(['/', 'Doujin', 'Fiction'], books)).toEqual(['/', 'Fiction']);
  });

  it('keeps a folder holding no book at all, which nobody hid', () => {
    expect(filter(['/', 'Empty'], [book('a', ['Fiction'])])).toEqual(['/', 'Empty']);
  });

  // A folder counts everything in its subtree, so a parent with one visible
  // book below it stays even when its own books are hidden.
  it('keeps an ancestor of a visible book', () => {
    const books = [book('a', ['Fiction'], { nsfw: true }), book('b', ['Fiction', 'Classics'])];

    expect(filter(['/', 'Fiction', 'Fiction/Classics'], books)).toEqual([
      '/',
      'Fiction',
      'Fiction/Classics'
    ]);
  });

  it('counts a book directly under books/ towards the top level', () => {
    expect(filter(['/', 'Fiction'], [book('a', [], { nsfw: true })])).toEqual(['Fiction']);
  });

  it('returns the folders untouched with the setting on', () => {
    const books = [book('a', ['Adult'], { nsfw: true })];

    expect(filter(['/', 'Adult'], books, true)).toEqual(['/', 'Adult']);
  });

  it('changes nothing on a shelf that marks nothing', () => {
    const folders = ['/', 'Empty', 'Fiction'];
    const books = [book('a', ['Fiction'])];

    expect(filter(folders, books)).toEqual(folders);
  });
});
