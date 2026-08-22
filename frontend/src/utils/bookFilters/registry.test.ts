import { describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import type { Book } from '@/types/book';

// The char-count dependency wraps useCharCountIndex; a controllable fake lets us
// exercise the load/augment/predicate contract without a backend.
const counts = ref<Map<string, number | undefined>>(new Map());
const ready = ref(false);
const load = vi.fn();

vi.mock('@/composables/useCharCountIndex', () => ({
  useCharCountIndex: () => ({ counts, ready, load })
}));

const {
  charCountFilter,
  authorFilter,
  coverFilter,
  languageFilter,
  tagsFilter
} = await import('./registry');

function book(id: string, charCount?: number): Book {
  return { id, title: id, authors: [], tags: [], layers: [], char_count: charCount };
}

function metaBook(overrides: Partial<Book> = {}): Book {
  return { id: 'b', title: 'b', authors: [], tags: [], layers: [], ...overrides };
}

describe('charCountFilter dependency', () => {
  it('augments a listing-omitted count from the loaded index so the predicate can decide it', () => {
    counts.value = new Map([['indexed', 500]]);
    ready.value = true;
    const dep = charCountFilter.createDependency!();
    const range = { min: 100, max: 900 };

    // Without the index the listing has no count, so the predicate reads 0 and
    // wrongly rejects the book; augmenting first applies the fetched 500.
    const listed = book('indexed');
    expect(charCountFilter.predicate(listed, range)).toBe(false);
    expect(charCountFilter.predicate(dep.augment(listed), range)).toBe(true);
  });

  it('leaves a book that already carries its own count untouched', () => {
    counts.value = new Map([['b', 999]]);
    const dep = charCountFilter.createDependency!();
    const withCount = book('b', 250);
    expect(dep.augment(withCount)).toBe(withCount);
  });

  it('counts only books whose count is unknown once the index is ready', () => {
    counts.value = new Map([['known', 300]]);
    ready.value = true;
    const dep = charCountFilter.createDependency!();
    const books = [book('known'), book('missing'), book('listed', 42)];

    expect(dep.unknownCount(books, { min: 100 })).toBe(1);
    // An inactive range decides nothing, so nothing is "unknown" to it.
    expect(dep.unknownCount(books, {})).toBe(0);
  });
});

describe('metadata field filters', () => {
  it('round-trips its URL token through parse and serialize', () => {
    for (const token of ['none', 'has', 'eq:魯迅'] as const) {
      const value = authorFilter.parse({ author: token });
      expect(value).toBeDefined();
      expect(authorFilter.serialize(value)).toEqual({ author: token });
    }
    // A hand-edited or stale token is ignored rather than throwing, and an
    // ignored value is inactive so it never narrows the list.
    const garbage = authorFilter.parse({ author: 'wat' });
    expect(garbage).toBeUndefined();
    expect(authorFilter.isActive(garbage)).toBe(false);
  });

  it('decides author by none/has/eq over the trimmed authors array', () => {
    const none = authorFilter.parse({ author: 'none' });
    const has = authorFilter.parse({ author: 'has' });
    const eq = authorFilter.parse({ author: 'eq:魯迅' });

    expect(authorFilter.predicate(metaBook({ authors: [] }), none)).toBe(true);
    expect(authorFilter.predicate(metaBook({ authors: ['  '] }), none)).toBe(true);
    expect(authorFilter.predicate(metaBook({ authors: ['魯迅'] }), none)).toBe(false);

    expect(authorFilter.predicate(metaBook({ authors: ['魯迅'] }), has)).toBe(true);
    expect(authorFilter.predicate(metaBook({ authors: [] }), has)).toBe(false);

    expect(authorFilter.predicate(metaBook({ authors: ['魯迅', '周樹人'] }), eq)).toBe(true);
    expect(authorFilter.predicate(metaBook({ authors: ['周作人'] }), eq)).toBe(false);
  });

  it('decides language by none/has/eq', () => {
    expect(languageFilter.predicate(metaBook({ language: '' }), languageFilter.parse({ language: 'none' }))).toBe(true);
    expect(languageFilter.predicate(metaBook({}), languageFilter.parse({ language: 'none' }))).toBe(true);
    expect(languageFilter.predicate(metaBook({ language: 'zh' }), languageFilter.parse({ language: 'has' }))).toBe(true);
    expect(languageFilter.predicate(metaBook({ language: 'zh' }), languageFilter.parse({ language: 'eq:zh' }))).toBe(true);
    expect(languageFilter.predicate(metaBook({ language: 'en' }), languageFilter.parse({ language: 'eq:zh' }))).toBe(false);
  });

  // Guards the cover contract: presence is decided by `cover` (the on-disk
  // filename), never by `cover_url`, which is derived downstream and absent on
  // pCloud even when a cover exists.
  it('decides cover by the cover filename, ignoring a derived cover_url', () => {
    const none = coverFilter.parse({ cover: 'none' });
    const has = coverFilter.parse({ cover: 'has' });

    expect(coverFilter.predicate(metaBook({ cover: '' }), none)).toBe(true);
    expect(coverFilter.predicate(metaBook({ cover: 'cover.jpg' }), none)).toBe(false);
    // A URL without a cover filename is still "missing a cover".
    expect(
      coverFilter.predicate(metaBook({ cover: '', cover_url: 'https://example/cover' }), none)
    ).toBe(true);
    expect(coverFilter.predicate(metaBook({ cover: 'cover.jpg' }), has)).toBe(true);
  });

  it('ANDs repeated tag values and defaults the operator to all', () => {
    const value = tagsFilter.parse({ tags: ['eq:待整理', 'eq:科幻'] });
    expect(value.op).toBe('all');
    expect(value.values).toHaveLength(2);
    expect(tagsFilter.isActive(value)).toBe(true);
    expect(tagsFilter.serialize(value)).toEqual({
      tags: ['eq:待整理', 'eq:科幻'],
      tagsOp: 'all'
    });

    expect(tagsFilter.predicate(metaBook({ tags: ['待整理', '科幻', '長篇'] }), value)).toBe(true);
    // AND: a book missing one of the required tags is excluded.
    expect(tagsFilter.predicate(metaBook({ tags: ['待整理'] }), value)).toBe(false);

    const none = tagsFilter.parse({ tags: 'none' });
    expect(tagsFilter.predicate(metaBook({ tags: [] }), none)).toBe(true);
    expect(tagsFilter.predicate(metaBook({ tags: ['待整理'] }), none)).toBe(false);
  });
});
