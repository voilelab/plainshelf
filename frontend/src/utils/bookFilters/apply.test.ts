import { describe, expect, it } from 'vitest';
import type { Book } from '@/types/book';
import { applyBookFilters, soleBlockingFilter, type ActiveBookFilter } from './apply';
import { authorFilter, coverFilter, tagsFilter } from './registry';

function book(overrides: Partial<Book> = {}): Book {
  return { id: 'b', title: 'b', authors: [], tags: [], folders: [], ...overrides };
}

const alice = book({ id: 'alice', authors: ['Alice'], cover: 'a.jpg', tags: ['fiction'] });
const bob = book({ id: 'bob', authors: ['Bob'], cover: '', tags: ['essays'] });
const carol = book({ id: 'carol', authors: [], cover: 'c.jpg', tags: [] });
const shelf = [alice, bob, carol];

function active(...entries: ActiveBookFilter[]): ActiveBookFilter[] {
  return entries;
}

describe('applyBookFilters', () => {
  it('returns everything when nothing is active', () => {
    expect(applyBookFilters(shelf, [])).toEqual(shelf);
  });

  it('ANDs every active condition', () => {
    const result = applyBookFilters(
      shelf,
      active(
        { filter: coverFilter, value: { kind: 'has' } },
        { filter: authorFilter, value: { kind: 'has' } }
      )
    );
    expect(result).toEqual([alice]);
  });
});

describe('soleBlockingFilter', () => {
  it('blames nothing while the list is non-empty', () => {
    const result = soleBlockingFilter(shelf, active({ filter: coverFilter, value: { kind: 'has' } }));
    expect(result).toBeNull();
  });

  it('names the one condition whose removal alone brings books back', () => {
    const result = soleBlockingFilter(
      shelf,
      active(
        { filter: authorFilter, value: { kind: 'eq', value: 'Zed' } },
        { filter: coverFilter, value: { kind: 'has' } }
      )
    );
    expect(result?.filter).toBe(authorFilter);
  });

  it('blames nothing when two conditions are jointly responsible', () => {
    const result = soleBlockingFilter(
      shelf,
      active(
        { filter: coverFilter, value: { kind: 'none' } },
        { filter: authorFilter, value: { kind: 'eq', value: 'Alice' } }
      )
    );
    expect(result).toBeNull();
  });

  it('blames the sole active condition when it empties the list', () => {
    const result = soleBlockingFilter(
      shelf,
      active({ filter: tagsFilter, value: { values: [{ kind: 'eq', value: 'poetry' }], op: 'all' } })
    );
    expect(result?.filter).toBe(tagsFilter);
  });
});
