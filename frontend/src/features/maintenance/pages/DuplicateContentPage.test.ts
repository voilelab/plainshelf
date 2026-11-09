// @vitest-environment jsdom
import { createApp, h, ref, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { ComicOverlapReport } from '@/api/books';

const mocks = vi.hoisted(() => ({
  getDuplicateBookGroups: vi.fn(),
  getComicOverlaps: vi.fn(),
  getBook: vi.fn(),
  startHashComicPages: vi.fn(),
  getTaskChain: vi.fn(),
  writable: { value: true }
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    getDuplicateBookGroups: mocks.getDuplicateBookGroups,
    getComicOverlaps: mocks.getComicOverlaps,
    getBook: mocks.getBook,
    getBookCoverUrl: () => ''
  }),
  bookshelfWriter: () => ({
    startHashComicPages: mocks.startHashComicPages,
    getTaskChain: mocks.getTaskChain
  })
}));

vi.mock('@/composables/useWriteAccess', () => {
  const writesEnabled = ref(true);
  mocks.writable = writesEnabled;
  return { useWriteAccess: () => ({ writesEnabled }) };
});

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }));

import DuplicateContentPage from './DuplicateContentPage.vue';
import { setLocale } from '@/i18n';

const EMPTY_REPORT: ComicOverlapReport = { total: 0, hashed: 0, missing: 0, overlaps: [] };

let mounted: { app: App; host: HTMLElement } | null = null;

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(DuplicateContentPage) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function buttonByText(host: HTMLElement, text: string): HTMLButtonElement {
  const button = [...host.querySelectorAll('button')].find((el) => el.textContent?.trim() === text);
  if (!button) throw new Error(`no button with text "${text}"`);
  return button as HTMLButtonElement;
}

beforeEach(() => {
  setLocale('en');
  mocks.getDuplicateBookGroups.mockReset().mockResolvedValue([]);
  mocks.getComicOverlaps.mockReset().mockResolvedValue(EMPTY_REPORT);
  mocks.getBook.mockReset().mockImplementation(async (id: string) => ({ id, title: `Title ${id}`, folder: [] }));
  mocks.startHashComicPages.mockReset().mockResolvedValue('chain-1');
  mocks.getTaskChain.mockReset().mockResolvedValue({ id: 'chain-1', status: 'running', percentage: 0, tasks: [] });
  mocks.writable.value = true;
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('DuplicateContentPage', () => {
  it('lists a comic overlap with how many pages each side has extra', async () => {
    mocks.getComicOverlaps.mockResolvedValue({
      total: 2,
      hashed: 2,
      missing: 0,
      overlaps: [{ a: 'c1', b: 'c2', pages_a: 10, pages_b: 12, shared: 10 }]
    });
    const host = mount();
    await flush();

    expect(host.textContent).toContain('Possible Duplicate #1');
    expect(host.textContent).toContain('10 pages in common; Title c1 has 0 more, Title c2 has 2 more');
    expect(host.querySelector('.duplicate-empty')).toBeNull();
  });

  it('hides the comic hash bar on a shelf without comics', async () => {
    const host = mount();
    await flush();

    expect(host.querySelector('.duplicate-comic-bar')).toBeNull();
    expect(host.querySelector('.duplicate-empty')).not.toBeNull();
  });

  it('starts the page hash task from the bar when comics are missing hashes', async () => {
    mocks.getComicOverlaps.mockResolvedValue({ ...EMPTY_REPORT, total: 5, hashed: 2, missing: 3 });
    const host = mount();
    await flush();

    expect(host.querySelector('.duplicate-comic-bar')!.textContent).toContain('3 of 5 comics have no page hashes yet');
    buttonByText(host, 'Hash comic pages').click();
    await flush();

    expect(mocks.startHashComicPages).toHaveBeenCalledTimes(1);
  });

  it('offers no hash button on a read-only shelf', async () => {
    mocks.writable.value = false;
    mocks.getComicOverlaps.mockResolvedValue({ ...EMPTY_REPORT, total: 5, hashed: 0, missing: 5 });
    const host = mount();
    await flush();

    expect(host.textContent).toContain('A read-only shelf cannot hash comic pages.');
    expect([...host.querySelectorAll('button')].some((b) => b.textContent?.trim() === 'Hash comic pages')).toBe(false);
  });
});
