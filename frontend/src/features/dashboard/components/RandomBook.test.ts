// @vitest-environment jsdom
import { defineComponent, h, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';

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
  getBookshelfProvider: () => ({}),
  isWebRuntime: launch.isWebRuntime
}));

vi.mock('@/composables/useReaderLaunchPreference', () => ({
  getReaderLaunchMode: launch.getReaderLaunchMode
}));

vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({
    props: { bookId: String, coverUrl: String, alt: String },
    setup: () => () => h('img')
  })
}));

import RandomBook from './RandomBook.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

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

function mountRandom(props: { books: Book[]; startedIds?: Set<string> }) {
  return mount(RandomBook, { props, components: { RouterLink: RouterLinkStub } });
}

// jsdom 30.1.1 Window.js leaves window.open unimplemented.
let openSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  setLocale('en');
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
    const { host } = mountRandom({ books, startedIds: new Set(['started']) });

    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('Fresh');
  });

  it('falls back to the whole shelf when every book has been started', () => {
    const books = [
      makeBook({ id: 'a', title: 'Alpha' }),
      makeBook({ id: 'b', title: 'Beta' })
    ];
    const { host } = mountRandom({ books, startedIds: new Set(['a', 'b']) });

    expect(host.querySelector('.random-book-empty')).toBeNull();
    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('Alpha');
  });

  it('advances on Shuffle even when the current book is the only unstarted one', async () => {
    const books = [
      makeBook({ id: 's1', title: 'StartedOne' }),
      makeBook({ id: 's2', title: 'StartedTwo' }),
      makeBook({ id: 'fresh', title: 'Fresh' })
    ];
    const { host } = mountRandom({ books, startedIds: new Set(['s1', 's2']) });

    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('Fresh');

    host.querySelector('button')?.click();
    await nextTick();
    expect(host.querySelector('.random-book-book-title')?.textContent).toBe('StartedOne');
  });

  it('opens the reader in a new tab under the new-reader preference and does not push', () => {
    const { host } = mountRandom({ books: [makeBook({ id: 'b1', title: 'Book' })] });

    (host.querySelector('.random-book-actions a.primary') as HTMLElement).click();

    expect(openSpy).toHaveBeenCalledWith('/reader/b1', '_blank', 'noopener,noreferrer');
    expect(launch.push).not.toHaveBeenCalled();
  });

  it('navigates in place under the in-window preference and does not open a new tab', () => {
    launch.getReaderLaunchMode.mockReturnValue('in-window');
    const { host } = mountRandom({ books: [makeBook({ id: 'b1', title: 'Book' })] });

    (host.querySelector('.random-book-actions a.primary') as HTMLElement).click();

    expect(launch.push).toHaveBeenCalledWith({ path: '/reader/b1' });
    expect(openSpy).not.toHaveBeenCalled();
  });
});
