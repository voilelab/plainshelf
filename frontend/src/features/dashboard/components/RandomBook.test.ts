// @vitest-environment jsdom
import { createApp, defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';

// BookCoverImg reaches the cover pipeline; the pick logic under test does not
// need it, so render a bare stub in its place.
vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({
    props: { bookId: String, coverUrl: String, alt: String },
    setup: () => () => h('img')
  })
}));

import RandomBook from './RandomBook.vue';
import { setLocale } from '@/i18n';

const RouterLinkStub = defineComponent({
  props: { to: { type: [String, Object], default: '' } },
  setup(props, { slots }) {
    return () => h('a', { 'data-to': JSON.stringify(props.to) }, slots.default?.());
  }
});

function makeBook(overrides: Partial<Book> = {}): Book {
  return { id: 'b1', title: 'Book', authors: [], tags: [], folders: [], ...overrides };
}

function mount(props: { books: Book[]; startedIds?: Set<string> }) {
  const host = document.createElement('div');
  const app = createApp(RandomBook, props);
  app.component('RouterLink', RouterLinkStub);
  app.mount(host);
  return { host, app };
}

beforeEach(() => {
  setLocale('en');
  // Deterministic pick: always the first element of whatever pool is chosen.
  vi.spyOn(Math, 'random').mockReturnValue(0);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('RandomBook', () => {
  it('prefers a book that has not been started', () => {
    const books = [
      makeBook({ id: 'started', title: 'Started' }),
      makeBook({ id: 'fresh', title: 'Fresh' })
    ];
    const { host, app } = mount({ books, startedIds: new Set(['started']) });

    // Index 0 of the unstarted pool is "Fresh"; the started book is skipped even
    // though it is first in the full list.
    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('Fresh');

    app.unmount();
  });

  it('falls back to the whole shelf when every book has been started', () => {
    const books = [
      makeBook({ id: 'a', title: 'Alpha' }),
      makeBook({ id: 'b', title: 'Beta' })
    ];
    const { host, app } = mount({ books, startedIds: new Set(['a', 'b']) });

    // No unstarted book to prefer, so it must still surface one rather than the
    // empty state — index 0 of the full list.
    expect(host.querySelector('.random-book-empty')).toBeNull();
    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('Alpha');

    app.unmount();
  });
});
