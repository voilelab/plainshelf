import { describe, expect, it } from 'vitest';
import type { Book } from '@/types/book';
import { bookMatchesSearch } from './bookSearch';

function makeBook(overrides: Partial<Book> = {}): Book {
  return {
    id: 'book-1',
    title: 'Untitled',
    authors: [],
    tags: [],
    folders: [],
    ...overrides
  };
}

describe('bookMatchesSearch', () => {
  it('returns all books when the query is empty', () => {
    const books = [makeBook({ id: '1' }), makeBook({ id: '2' })];
    expect(books.filter((book) => bookMatchesSearch(book, ''))).toEqual(books);
  });

  it('returns all books when the query is only whitespace', () => {
    const books = [makeBook({ id: '1' }), makeBook({ id: '2' })];
    expect(books.filter((book) => bookMatchesSearch(book, '   '))).toEqual(books);
  });

  it('matches on title', () => {
    const books = [
      makeBook({ id: '1', title: 'The Quiet River' }),
      makeBook({ id: '2', title: 'Mountain Diary' })
    ];
    expect(books.filter((book) => bookMatchesSearch(book, 'quiet')).map((b) => b.id)).toEqual(['1']);
  });

  it('matches on authors', () => {
    const books = [
      makeBook({ id: '1', authors: ['Ada Lovelace'] }),
      makeBook({ id: '2', authors: ['Grace Hopper'] })
    ];
    expect(books.filter((book) => bookMatchesSearch(book, 'hopper')).map((b) => b.id)).toEqual(['2']);
  });

  it('matches on tags', () => {
    const books = [
      makeBook({ id: '1', tags: ['fiction', 'calm'] }),
      makeBook({ id: '2', tags: ['programming'] })
    ];
    expect(books.filter((book) => bookMatchesSearch(book, 'programming')).map((b) => b.id)).toEqual(['2']);
  });

  it('matches on comment', () => {
    const books = [
      makeBook({ id: '1', comment: 'Imported from local txt file.' }),
      makeBook({ id: '2', comment: 'Nothing special here.' })
    ];
    expect(books.filter((book) => bookMatchesSearch(book, 'imported')).map((b) => b.id)).toEqual(['1']);
  });

  it('is case-insensitive', () => {
    const books = [makeBook({ id: '1', title: 'The Quiet River' })];
    expect(books.filter((book) => bookMatchesSearch(book, 'QUIET')).map((b) => b.id)).toEqual(['1']);
    expect(books.filter((book) => bookMatchesSearch(book, 'the quiet river')).map((b) => b.id)).toEqual(['1']);
  });

  it('trims the query before matching', () => {
    const books = [makeBook({ id: '1', title: 'The Quiet River' })];
    expect(books.filter((book) => bookMatchesSearch(book, '  quiet  ')).map((b) => b.id)).toEqual(['1']);
  });

  it('returns an empty array when nothing matches', () => {
    const books = [makeBook({ id: '1', title: 'The Quiet River' })];
    expect(books.filter((book) => bookMatchesSearch(book, 'nonexistent-term'))).toEqual([]);
  });

  it('handles books with no comment', () => {
    const books = [makeBook({ id: '1', title: 'No Comment Book', comment: undefined })];
    expect(books.filter((book) => bookMatchesSearch(book, 'no comment')).map((b) => b.id)).toEqual(['1']);
    expect(books.filter((book) => bookMatchesSearch(book, 'nonexistent'))).toEqual([]);
  });
});
