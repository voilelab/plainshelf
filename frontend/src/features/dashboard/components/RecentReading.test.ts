// @vitest-environment jsdom
import { defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';
import type { RecentReadingItem } from '@/features/dashboard/composables/useDashboardData';

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

import RecentReading from './RecentReading.vue';
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
  return { id: 'b1', title: 'Book', authors: [], tags: [], folders: [], ...overrides } as Book;
}

function makeItem(overrides: Partial<RecentReadingItem> = {}): RecentReadingItem {
  return { book: makeBook(), percent: null, lastReadAt: null, ...overrides };
}

function mountRecent(props: { items: RecentReadingItem[] }) {
  return mount(RecentReading, { props, components: { RouterLink: RouterLinkStub } });
}

// jsdom 30.1.1 Window.js leaves window.open unimplemented.
let openSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  setLocale('en');
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

describe('RecentReading', () => {
  it('opens the reader in a new tab under the new-reader preference and does not push', () => {
    const { host } = mountRecent({ items: [makeItem({ book: makeBook({ id: 'b1' }) })] });

    (host.querySelector('a.recent-reading-card') as HTMLElement).click();

    expect(openSpy).toHaveBeenCalledWith('/reader/b1', '_blank', 'noopener,noreferrer');
    expect(launch.push).not.toHaveBeenCalled();
  });

  it('navigates in place under the in-window preference and does not open a new tab', () => {
    launch.getReaderLaunchMode.mockReturnValue('in-window');
    const { host } = mountRecent({ items: [makeItem({ book: makeBook({ id: 'b1' }) })] });

    (host.querySelector('a.recent-reading-card') as HTMLElement).click();

    expect(launch.push).toHaveBeenCalledWith({ path: '/reader/b1' });
    expect(openSpy).not.toHaveBeenCalled();
  });
});
