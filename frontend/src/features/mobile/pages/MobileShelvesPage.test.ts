// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  loadShelfEntries: vi.fn(),
  deleteShelfEntry: vi.fn(),
  listShelves: vi.fn(),
  removeShelf: vi.fn(),
  reloadIntoApp: vi.fn()
}));

vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ push: vi.fn() }) }));
vi.mock('@/providers/mobileConfig', () => ({
  loadShelfEntries: mocks.loadShelfEntries,
  deleteShelfEntry: mocks.deleteShelfEntry,
  setActiveShelfEntry: vi.fn(),
  shelfEntryDisplayName: (entry: { name: string }) => entry.name,
  SOURCE_TYPE_LABEL_KEYS: { server: 'mobileConnect.modeServer', pcloud: 'mobileConnect.modePCloud', local: 'mobileConnect.modeLocal' }
}));
vi.mock('@/providers/localShelf', () => ({
  LocalShelf: { listShelves: mocks.listShelves, removeShelf: mocks.removeShelf }
}));
vi.mock('@/providers/mobileCacheFs', () => ({ removeCacheScope: vi.fn() }));
vi.mock('@/providers/cacheScope', () => ({ cacheScopeKeyForEntry: () => '' }));
vi.mock('@/features/mobile/utils/reloadIntoApp', () => ({ reloadIntoApp: mocks.reloadIntoApp }));
vi.mock('@/features/mobile/components/MobileShelfForm.vue', () => ({ default: { render: () => null } }));

import MobileShelvesPage from './MobileShelvesPage.vue';

const local = { id: 'e1', type: 'local', name: 'Phone', localShelfId: 'local-1' };
const server = { id: 'e2', type: 'server', name: 'Home', serverUrl: 'http://h', shelfId: 's' };

let mounted: { app: App; host: HTMLElement } | null = null;

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

async function removeFirst(): Promise<HTMLElement> {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(MobileShelvesPage) });
  app.mount(host);
  mounted = { app, host };
  await flush();
  const buttons = [...host.querySelectorAll<HTMLButtonElement>('.mobile-shelves-item button')];
  buttons.find((b) => b.textContent?.trim() === 'Remove')!.click();
  await flush();
  host.querySelector<HTMLButtonElement>('.mobile-shelves-confirm .button.primary')!.click();
  await flush();
  return host;
}

beforeEach(() => {
  mocks.removeShelf.mockReset().mockResolvedValue(undefined);
  mocks.deleteShelfEntry.mockReset().mockResolvedValue(undefined);
  mocks.loadShelfEntries.mockResolvedValue({ entries: [local, server], activeEntryID: 'e1' });
  mocks.listShelves.mockResolvedValue({ shelves: [{ id: 'local-1', name: 'Phone' }] });
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  vi.clearAllMocks();
});

describe('removing a local shelf', () => {
  it('keeps the entry, and the running app on it, when the removal fails', async () => {
    mocks.removeShelf.mockRejectedValue(new Error('plainshelf: background tasks are still running'));

    const host = await removeFirst();

    expect(mocks.deleteShelfEntry).not.toHaveBeenCalled();
    expect(mocks.reloadIntoApp).not.toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-error')?.textContent).toContain('still running');
  });

  it('removes the shelf before its entry, then restarts off it', async () => {
    const host = await removeFirst();

    expect(mocks.removeShelf).toHaveBeenCalledWith({ id: 'local-1' });
    expect(mocks.removeShelf.mock.invocationCallOrder[0]).toBeLessThan(
      mocks.deleteShelfEntry.mock.invocationCallOrder[0]
    );
    expect(mocks.reloadIntoApp).toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-error')).toBeNull();
  });

  it('drops an entry whose shelf is already gone', async () => {
    mocks.listShelves.mockResolvedValue({ shelves: [] });

    await removeFirst();

    expect(mocks.removeShelf).not.toHaveBeenCalled();
    expect(mocks.deleteShelfEntry).toHaveBeenCalledWith('e1');
  });
});
