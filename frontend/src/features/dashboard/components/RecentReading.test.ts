// @vitest-environment jsdom
import { createApp, defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';
import type { RecentReadingItem } from '@/features/dashboard/composables/useDashboardData';

// Reader launch reaches vue-router, the runtime provider and the device-local
// preference; stub all three so a card click can be exercised deterministically.
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

// BookCoverImg reaches the cover pipeline; the click path under test does not
// need it, so render a bare stub in its place.
vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({
    props: { bookId: String, coverUrl: String, alt: String },
    setup: () => () => h('img')
  })
}));

import RecentReading from './RecentReading.vue';
import { setLocale } from '@/i18n';

// Mirrors RouterLink: a plain link renders an <a>; `custom` renders the slot
// content instead, handing it the resolved `href` the card wires onto its own
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
  return { id: 'b1', title: 'Book', authors: [], tags: [], folders: [], ...overrides } as Book;
}

function makeItem(overrides: Partial<RecentReadingItem> = {}): RecentReadingItem {
  return { book: makeBook(), percent: null, lastReadAt: null, ...overrides };
}

function mount(props: { items: RecentReadingItem[] }) {
  const host = document.createElement('div');
  const app = createApp(RecentReading, props);
  app.component('RouterLink', RouterLinkStub);
  app.mount(host);
  return { host, app };
}

// window.open is unimplemented in jsdom; a spy silences it and lets the launch
// tests assert the new-tab call.
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
  // The card now honours the reader-launch preference instead of always
  // navigating in place: on a web build with 'new-reader' it opens a new tab and
  // must not push the current window.
  it('opens the reader in a new tab under the new-reader preference and does not push', () => {
    const { host, app } = mount({ items: [makeItem({ book: makeBook({ id: 'b1' }) })] });

    (host.querySelector('a.recent-reading-card') as HTMLElement).click();

    expect(openSpy).toHaveBeenCalledWith('/reader/b1', '_blank', 'noopener,noreferrer');
    expect(launch.push).not.toHaveBeenCalled();

    app.unmount();
  });

  // Reverse: with 'in-window' it navigates in place and must not open a new tab.
  it('navigates in place under the in-window preference and does not open a new tab', () => {
    launch.getReaderLaunchMode.mockReturnValue('in-window');
    const { host, app } = mount({ items: [makeItem({ book: makeBook({ id: 'b1' }) })] });

    (host.querySelector('a.recent-reading-card') as HTMLElement).click();

    expect(launch.push).toHaveBeenCalledWith({ path: '/reader/b1' });
    expect(openSpy).not.toHaveBeenCalled();

    app.unmount();
  });
});
