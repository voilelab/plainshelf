import { describe, expect, it } from 'vitest';
import type { Book } from '@/types/book';
import { filterBooksBySearch } from './bookSearch';

function makeBook(overrides: Partial<Book> = {}): Book {
  return {
    id: 'book-1',
    title: 'Untitled',
    authors: [],
    tags: [],
    layers: [],
    ...overrides
  };
}

describe('filterBooksBySearch', () => {
  it('returns all books when the query is empty', () => {
    const books = [makeBook({ id: '1' }), makeBook({ id: '2' })];
    expect(filterBooksBySearch(books, '')).toEqual(books);
  });

  it('returns all books when the query is only whitespace', () => {
    const books = [makeBook({ id: '1' }), makeBook({ id: '2' })];
    expect(filterBooksBySearch(books, '   ')).toEqual(books);
  });

  it('matches on title', () => {
    const books = [
      makeBook({ id: '1', title: 'The Quiet River' }),
      makeBook({ id: '2', title: 'Mountain Diary' })
    ];
    expect(filterBooksBySearch(books, 'quiet').map((b) => b.id)).toEqual(['1']);
  });

  it('matches on authors', () => {
    const books = [
      makeBook({ id: '1', authors: ['Ada Lovelace'] }),
      makeBook({ id: '2', authors: ['Grace Hopper'] })
    ];
    expect(filterBooksBySearch(books, 'hopper').map((b) => b.id)).toEqual(['2']);
  });

  it('matches on tags', () => {
    const books = [
      makeBook({ id: '1', tags: ['fiction', 'calm'] }),
      makeBook({ id: '2', tags: ['programming'] })
    ];
    expect(filterBooksBySearch(books, 'programming').map((b) => b.id)).toEqual(['2']);
  });

  it('matches on comment', () => {
    const books = [
      makeBook({ id: '1', comment: 'Imported from local txt file.' }),
      makeBook({ id: '2', comment: 'Nothing special here.' })
    ];
    expect(filterBooksBySearch(books, 'imported').map((b) => b.id)).toEqual(['1']);
  });

  it('is case-insensitive', () => {
    const books = [makeBook({ id: '1', title: 'The Quiet River' })];
    expect(filterBooksBySearch(books, 'QUIET').map((b) => b.id)).toEqual(['1']);
    expect(filterBooksBySearch(books, 'the quiet river').map((b) => b.id)).toEqual(['1']);
  });

  it('trims the query before matching', () => {
    const books = [makeBook({ id: '1', title: 'The Quiet River' })];
    expect(filterBooksBySearch(books, '  quiet  ').map((b) => b.id)).toEqual(['1']);
  });

  it('returns an empty array when nothing matches', () => {
    const books = [makeBook({ id: '1', title: 'The Quiet River' })];
    expect(filterBooksBySearch(books, 'nonexistent-term')).toEqual([]);
  });

  it('handles books with no comment', () => {
    const books = [makeBook({ id: '1', title: 'No Comment Book', comment: undefined })];
    expect(filterBooksBySearch(books, 'no comment').map((b) => b.id)).toEqual(['1']);
    expect(filterBooksBySearch(books, 'nonexistent')).toEqual([]);
  });
});
