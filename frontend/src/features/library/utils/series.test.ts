import { describe, expect, it } from 'vitest';
import type { Book } from '@/types/book';
import { booksInSeries, orderForNumbering, summarizeSeries } from './series';

function book(title: string, series?: Book['series']): Book {
  return { id: title, title, authors: [], tags: [], folders: [], series };
}

describe('summarizeSeries', () => {
  it('counts each series once, sorted by name with numbers in numeric order', () => {
    expect(
      summarizeSeries([
        book('a', { name: 'Saga 10', index: 1 }),
        book('b', { name: 'Saga 2', index: 1 }),
        book('c'),
        book('d', { name: 'Saga 2', index: 2 }),
        book('e', { name: 'Atlas', index: 1 })
      ]).map(({ name, count }) => ({ name, count }))
    ).toEqual([
      { name: 'Atlas', count: 1 },
      { name: 'Saga 2', count: 2 },
      { name: 'Saga 10', count: 1 }
    ]);
  });

  it('keeps names that differ only in case apart, and folds surrounding spaces', () => {
    expect(
      summarizeSeries([
        book('a', { name: 'saga', index: 1 }),
        book('b', { name: 'Saga', index: 1 }),
        book('c', { name: ' Saga ', index: 2 })
      ]).map(({ name, count }) => `${name}:${count}`)
    ).toEqual(expect.arrayContaining(['saga:1', 'Saga:2']));
  });

  it('is empty for a shelf with no series', () => {
    expect(summarizeSeries([book('a'), book('b')])).toEqual([]);
  });

  it('takes the cover of the lowest-numbered book that has one', () => {
    const covered = (title: string, index: number): Book => ({
      ...book(title, { name: 'Saga', index }),
      cover_url: `/covers/${title}`
    });

    const [summary] = summarizeSeries([covered('Vol 3', 3), book('Vol 1', { name: 'Saga', index: 1 }), covered('Vol 2', 2)]);

    expect(summary.coverBook.title).toBe('Vol 2');
  });

  it('falls back to the first book when none has a cover', () => {
    const [summary] = summarizeSeries([book('Vol 2', { name: 'Saga', index: 2 }), book('Vol 1', { name: 'Saga', index: 1 })]);

    expect(summary.coverBook.title).toBe('Vol 1');
  });
});

describe('booksInSeries', () => {
  it('orders by number, fractions between volumes, then by title on a tie', () => {
    const books = [
      book('Vol 2', { name: 'Saga', index: 2 }),
      book('Side B', { name: 'Saga', index: 1.5 }),
      book('Vol 1', { name: 'Saga', index: 1 }),
      book('Side A', { name: 'Saga', index: 1.5 }),
      book('Prologue', { name: 'Saga', index: 0 }),
      book('Other', { name: 'Atlas', index: 1 }),
      book('Loose')
    ];

    expect(booksInSeries(books, 'Saga').map((b) => b.title)).toEqual([
      'Prologue',
      'Vol 1',
      'Side A',
      'Side B',
      'Vol 2'
    ]);
  });

  it('matches the name exactly after trimming', () => {
    const books = [book('a', { name: 'Saga', index: 1 }), book('b', { name: 'saga', index: 1 })];

    expect(booksInSeries(books, ' Saga ').map((b) => b.title)).toEqual(['a']);
    expect(booksInSeries(books, 'Sag')).toEqual([]);
  });
});

describe('orderForNumbering', () => {
  it('orders titles naturally, so 10 comes after 2', () => {
    expect(orderForNumbering([book('xxx 10'), book('xxx 2'), book('xxx 1')]).map((b) => b.title)).toEqual([
      'xxx 1',
      'xxx 2',
      'xxx 10'
    ]);
  });
});
