// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Book } from '@/types/book';

// The page reads two things off the provider: the downloaded entries and
// whether this backend is one that applies the device setting itself.
const mocks = vi.hoisted(() => ({
  listDownloadedBookEntries: vi.fn(),
  getStorageEstimate: vi.fn(),
  filtersNsfwOnDevice: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    listDownloadedBookEntries: mocks.listDownloadedBookEntries,
    getStorageEstimate: mocks.getStorageEstimate,
    filtersNsfwOnDevice: mocks.filtersNsfwOnDevice
  })
}));

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }));

import DownloadsPage from './DownloadsPage.vue';
import { setShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';

function book(id: string, mark: Partial<Pick<Book, 'nsfw' | 'nsfw_folder'>> = {}): Book {
  return { id, title: id, authors: [], tags: [], folders: [], ...mark };
}

const DOWNLOADS = [
  { book: book('plain'), sizeBytes: 10, downloadedAt: 1 },
  { book: book('own-mark', { nsfw: true }), sizeBytes: 100, downloadedAt: 2 },
  { book: book('by-folder', { nsfw_folder: { path: 'Adult' } }), sizeBytes: 1000, downloadedAt: 3 }
];

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
  mocks.listDownloadedBookEntries.mockResolvedValue(DOWNLOADS);
  mocks.getStorageEstimate.mockResolvedValue({ supported: false });
  mocks.filtersNsfwOnDevice.mockReturnValue(true);
  setShowNsfwOnDevice(false);
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  setShowNsfwOnDevice(false);
  vi.clearAllMocks();
});

describe('DownloadsPage adult content', () => {
  // Already being on the device is not a way past the setting: otherwise "hidden
  // on this phone" would hold everywhere but the one list that works offline.
  it('hides the marked books while the device setting is off', async () => {
    const host = mount();
    await flush();

    expect(titles(host)).toEqual(['plain']);
  });

  it('leaves the hidden books out of the count and the size total', async () => {
    const host = mount();
    await flush();

    // One book and 10 bytes, not three and 1110: the overview must not describe
    // what it does not list.
    const overview = host.querySelector('.downloads-overview')?.textContent ?? '';
    expect(overview).toContain('1 downloaded');
    expect(overview).toContain('10 B');
  });

  it('lists them again once the setting is on, without re-reading the device', async () => {
    const host = mount();
    await flush();

    setShowNsfwOnDevice(true);
    await flush();

    expect(titles(host)).toEqual(['plain', 'own-mark', 'by-folder']);
    expect(mocks.listDownloadedBookEntries).toHaveBeenCalledTimes(1);
  });

  // The reverse case: behind a PlainShelf server the listing was already
  // filtered by that server's own show_nsfw, and this device setting must not
  // be allowed to overrule it in either direction.
  it('filters nothing when the backend has a server to answer for it', async () => {
    mocks.filtersNsfwOnDevice.mockReturnValue(false);

    const host = mount();
    await flush();

    expect(titles(host)).toEqual(['plain', 'own-mark', 'by-folder']);
  });
});
