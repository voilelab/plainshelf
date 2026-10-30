import { describe, expect, it } from 'vitest';
import { mockBooks, mockCopyBook, mockGetBook, mockGetBookContent } from './books';

describe('mock book listing shape', () => {
  it('carries a cover filename wherever it carries a cover_url', () => {
    const inconsistent = mockBooks
      .filter((book) => book.cover_url)
      .filter((book) => !(typeof book.cover === 'string' && book.cover.trim().length > 0))
      .map((book) => book.id);

    expect(inconsistent).toEqual([]);
  });
});

describe('mockCopyBook', () => {
  it('gives the copy a fresh id in the target folder', () => {
    const copy = mockCopyBook('book-1', 'fiction/copies');

    expect(copy.id).not.toBe('book-1');
    expect(copy.folders).toEqual(['fiction', 'copies']);
    expect(mockGetBook(copy.id).id).toBe(copy.id);
  });

  it('carries the source content across to the copy', () => {
    const original = mockGetBookContent('book-1').content;
    const copy = mockCopyBook('book-1', '');

    expect(mockGetBookContent(copy.id).content).toBe(original);
  });
});
