// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';

const mocks = vi.hoisted(() => ({
  listDownloadedBookEntries: vi.fn(),
  getStorageEstimate: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    listDownloadedBookEntries: mocks.listDownloadedBookEntries,
    getStorageEstimate: mocks.getStorageEstimate
  })
}));

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }));

import DownloadsPage from './DownloadsPage.vue';
import { setShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';

function book(id: string): Book {
  return { id, title: id, authors: [], tags: [], folders: [] };
}

let mounted: { app: App; host: HTMLElement } | null = null;

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(DownloadsPage) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function titles(host: HTMLElement): string[] {
  return [...host.querySelectorAll('.book-list-title')].map((el) => el.textContent?.trim() ?? '');
}

beforeEach(() => {
  mocks.getStorageEstimate.mockResolvedValue({ supported: false });
  setShowNsfwOnDevice(false);
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  setShowNsfwOnDevice(false);
  vi.clearAllMocks();
});

describe('DownloadsPage', () => {
  it('lists what the provider hands it, and totals only that', async () => {
    mocks.listDownloadedBookEntries.mockResolvedValue([
      { book: book('plain'), sizeBytes: 10, downloadedAt: 1 }
    ]);

    const host = mount();
    await flush();

    expect(titles(host)).toEqual(['plain']);
    const overview = host.querySelector('.downloads-overview')?.textContent ?? '';
    expect(overview).toContain('1 downloaded');
    expect(overview).toContain('10 B');
  });

  // The provider withholds the books this device hides (see
  // VisibleMobileBookCache), so the page's job is only to ask again when the
  // answer changes — the list is otherwise fetched once, on mount.
  it('refetches when the device adult-content setting changes', async () => {
    mocks.listDownloadedBookEntries.mockResolvedValue([
      { book: book('plain'), sizeBytes: 10, downloadedAt: 1 }
    ]);

    const host = mount();
    await flush();
    expect(mocks.listDownloadedBookEntries).toHaveBeenCalledTimes(1);

    mocks.listDownloadedBookEntries.mockResolvedValue([
      { book: book('plain'), sizeBytes: 10, downloadedAt: 1 },
      { book: book('marked'), sizeBytes: 100, downloadedAt: 2 }
    ]);
    setShowNsfwOnDevice(true);
    await flush();

    expect(mocks.listDownloadedBookEntries).toHaveBeenCalledTimes(2);
    expect(titles(host)).toEqual(['plain', 'marked']);
  });
});
