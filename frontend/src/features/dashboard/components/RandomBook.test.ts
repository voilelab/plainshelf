// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';

// Reader launch reaches vue-router, the runtime provider and the device-local
// preference; stub all three so "Read now" can be exercised deterministically.
const launch = vi.hoisted(() => ({
  push: vi.fn(),
  resolve: vi.fn((to: unknown) => ({
    href: typeof to === 'string' ? to : (to as { path: string }).path
  })),
  isWebRuntime: vi.fn(() => true),
  getReaderLaunchMode: vi.fn(() => 'new-reader' as 'new-reader' | 'in-window')
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: launch.push, resolve: launch.resolve })
}));

vi.mock('@/providers', () => ({
  // No desktop reader on this web build, so launch stays on the window.open path.
  getBookshelfProvider: () => ({}),
  isWebRuntime: launch.isWebRuntime
}));

vi.mock('@/composables/useReaderLaunchPreference', () => ({
  getReaderLaunchMode: launch.getReaderLaunchMode
}));

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

// Mirrors RouterLink: a plain link renders an <a>; `custom` renders the slot
// content instead, handing it the resolved `href` the entry wires onto its own
// <a>.
const RouterLinkStub = defineComponent({
  props: { to: { type: [String, Object], default: '' }, custom: Boolean },
  setup(props, { slots }) {
    const href = () => (typeof props.to === 'string' ? props.to : (props.to as { path: string }).path);
    return () =>
      props.custom
        ? slots.default?.({ href: href(), navigate: () => {}, isActive: false, isExactActive: false })
        : h('a', { href: href(), 'data-to': JSON.stringify(props.to) }, slots.default?.());
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

// window.open is unimplemented in jsdom; a spy silences it and lets the launch
// tests assert the new-tab call.
let openSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  setLocale('en');
  // Deterministic pick: always the first element of whatever pool is chosen.
  vi.spyOn(Math, 'random').mockReturnValue(0);
  launch.push.mockReset();
  launch.resolve.mockReset();
  launch.resolve.mockImplementation((to: unknown) => ({
    href: typeof to === 'string' ? to : (to as { path: string }).path
  }));
  launch.isWebRuntime.mockReturnValue(true);
  launch.getReaderLaunchMode.mockReturnValue('new-reader');
  openSpy = vi.spyOn(window, 'open').mockReturnValue({} as Window);
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

  it('advances on Shuffle even when the current book is the only unstarted one', async () => {
    // The button is enabled on total book count, so a shelf with several books
    // but a single unstarted one (which becomes the initial pick) must still move
    // off it — otherwise Shuffle looks broken.
    const books = [
      makeBook({ id: 's1', title: 'StartedOne' }),
      makeBook({ id: 's2', title: 'StartedTwo' }),
      makeBook({ id: 'fresh', title: 'Fresh' })
    ];
    const { host, app } = mount({ books, startedIds: new Set(['s1', 's2']) });

    // Initial pick prefers the sole unstarted book.
    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('Fresh');

    // Shuffle falls back to the started books (index 0 of the current-excluded list).
    host.querySelector('button')?.click();
    await nextTick();
    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('StartedOne');

    app.unmount();
  });

  // "Read now" now honours the reader-launch preference instead of always
  // navigating in place: on a web build with 'new-reader' it opens a new tab and
  // must not push the current window.
  it('opens the reader in a new tab under the new-reader preference and does not push', () => {
    const { host, app } = mount({ books: [makeBook({ id: 'b1', title: 'Book' })] });

    (host.querySelector('.random-book-actions a.primary') as HTMLElement).click();

    expect(openSpy).toHaveBeenCalledWith('/reader/b1', '_blank', 'noopener,noreferrer');
    expect(launch.push).not.toHaveBeenCalled();

    app.unmount();
  });

  // Reverse: with 'in-window' it navigates in place and must not open a new tab.
  it('navigates in place under the in-window preference and does not open a new tab', () => {
    launch.getReaderLaunchMode.mockReturnValue('in-window');
    const { host, app } = mount({ books: [makeBook({ id: 'b1', title: 'Book' })] });

    (host.querySelector('.random-book-actions a.primary') as HTMLElement).click();

    expect(launch.push).toHaveBeenCalledWith({ path: '/reader/b1' });
    expect(openSpy).not.toHaveBeenCalled();

    app.unmount();
  });
});
