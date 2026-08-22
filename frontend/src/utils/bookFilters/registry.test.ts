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

const { charCountFilter } = await import('./registry');

function book(id: string, charCount?: number): Book {
  return { id, title: id, authors: [], tags: [], layers: [], char_count: charCount };
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
