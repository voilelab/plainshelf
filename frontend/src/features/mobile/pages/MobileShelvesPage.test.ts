// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  loadShelfEntries: vi.fn(),
  deleteShelfEntry: vi.fn(),
  listShelves: vi.fn(),
  removeShelf: vi.fn(),
  exportShelf: vi.fn(),
  importShelf: vi.fn(),
  addShelfEntry: vi.fn(),
  upsertShelfEntry: vi.fn(),
  isRestorePromptDismissed: vi.fn(),
  dismissRestorePrompt: vi.fn(),
  setActiveShelfEntry: vi.fn(),
  reloadIntoApp: vi.fn()
}));

vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ push: vi.fn() }) }));
vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => true }, registerPlugin: () => ({}) }));
vi.mock('@/providers/mobileConfig', () => ({
  loadShelfEntries: mocks.loadShelfEntries,
  deleteShelfEntry: mocks.deleteShelfEntry,
  addShelfEntry: mocks.addShelfEntry,
  upsertShelfEntry: mocks.upsertShelfEntry,
  isRestorePromptDismissed: mocks.isRestorePromptDismissed,
  dismissRestorePrompt: mocks.dismissRestorePrompt,
  newShelfEntry: () => ({ id: 'new-entry', type: 'local', name: '', localShelfId: '' }),
  setActiveShelfEntry: mocks.setActiveShelfEntry,
  shelfEntryDisplayName: (entry: { name: string }) => entry.name,
  SOURCE_TYPE_LABEL_KEYS: { server: 'mobileConnect.modeServer', pcloud: 'mobileConnect.modePCloud', local: 'mobileConnect.modeLocal' }
}));
// The real helpers, so the Go error wording is matched as in the app.
vi.mock('@/providers/localShelf', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/providers/localShelf')>()),
  LocalShelf: {
    listShelves: mocks.listShelves,
    removeShelf: mocks.removeShelf,
    exportShelf: mocks.exportShelf,
    importShelf: mocks.importShelf
  }
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

async function mountPage(): Promise<HTMLElement> {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(MobileShelvesPage) });
  app.mount(host);
  mounted = { app, host };
  await flush();
  return host;
}

function button(host: HTMLElement, selector: string): HTMLButtonElement {
  return host.querySelector<HTMLButtonElement>(selector)!;
}

async function removeFirst(): Promise<HTMLElement> {
  const host = await mountPage();
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
  mocks.listShelves.mockReset().mockResolvedValue({ shelves: [{ id: 'local-1', name: 'Phone' }] });
  mocks.exportShelf.mockReset().mockResolvedValue({ cancelled: false });
  mocks.importShelf.mockReset().mockResolvedValue({ cancelled: false, id: 'local-2', name: 'Backup' });
  mocks.addShelfEntry.mockReset().mockResolvedValue(undefined);
  mocks.setActiveShelfEntry.mockReset().mockResolvedValue(undefined);
  mocks.upsertShelfEntry.mockReset().mockResolvedValue(undefined);
  mocks.isRestorePromptDismissed.mockReset().mockResolvedValue(false);
  mocks.dismissRestorePrompt.mockReset().mockResolvedValue(undefined);
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

describe('exporting a local shelf', () => {
  it('offers export only for the local shelf and shows it has never been exported', async () => {
    const host = await mountPage();

    expect(host.querySelectorAll('.mobile-shelves-export')).toHaveLength(1);
    expect(host.querySelector('.mobile-shelves-last-export')?.textContent).toContain('Not backed up yet');
  });

  it('exports under a dated name and shows the new last export time', async () => {
    const host = await mountPage();
    mocks.listShelves.mockResolvedValue({
      shelves: [{ id: 'local-1', name: 'Phone', last_export: '2026-10-17T03:04:05Z' }]
    });

    button(host, '.mobile-shelves-export').click();
    await flush();

    expect(mocks.exportShelf).toHaveBeenCalledWith({
      id: 'local-1',
      fileName: expect.stringMatching(/^plainshelf-Phone-\d{8}\.zip$/)
    });
    expect(host.querySelector('.mobile-shelves-last-export')?.textContent).toContain('Last exported');
    expect(host.querySelector('.mobile-shelves-notice')).not.toBeNull();
    expect(host.querySelector('.mobile-shelves-error')).toBeNull();
  });

  it('changes nothing when the picker is cancelled', async () => {
    mocks.exportShelf.mockResolvedValue({ cancelled: true });
    const host = await mountPage();

    button(host, '.mobile-shelves-export').click();
    await flush();

    expect(mocks.listShelves).toHaveBeenCalledTimes(1);
    expect(host.querySelector('.mobile-shelves-last-export')?.textContent).toContain('Not backed up yet');
    expect(host.querySelector('.mobile-shelves-notice')).toBeNull();
    expect(host.querySelector('.mobile-shelves-error')).toBeNull();
  });

  it('asks to try again later while background tasks run', async () => {
    mocks.exportShelf.mockRejectedValue(new Error('mobile.ExportShelf: plainshelf: background tasks are still running'));
    const host = await mountPage();

    button(host, '.mobile-shelves-export').click();
    await flush();

    expect(host.querySelector('.mobile-shelves-error')?.textContent).toContain('try again shortly');
    expect(host.querySelector('.mobile-shelves-last-export')?.textContent).toContain('Not backed up yet');
  });
});

describe('importing a shelf zip', () => {
  it('adds the zip as a new local shelf without switching to it', async () => {
    const host = await mountPage();

    button(host, '.mobile-shelves-import').click();
    await flush();

    expect(mocks.addShelfEntry).toHaveBeenCalledWith({
      id: 'new-entry',
      type: 'local',
      name: 'Backup',
      localShelfId: 'local-2'
    });
    expect(mocks.setActiveShelfEntry).not.toHaveBeenCalled();
    expect(mocks.reloadIntoApp).not.toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-notice')?.textContent).toContain('Backup');
  });

  it.each([
    ['not a PlainShelf shelf: the zip has no books/ directory', 'no books/ folder'],
    ['zip entry escapes the shelf: "../evil"', 'outside the shelf'],
    ['zip unpacks to more than 512 MiB', '512 MiB'],
    ['plainshelf: background tasks are still running', 'try again shortly']
  ])('rejects %j with its reason and records no shelf', async (message, shown) => {
    mocks.importShelf.mockRejectedValue(new Error(message));
    const host = await mountPage();

    button(host, '.mobile-shelves-import').click();
    await flush();

    expect(mocks.addShelfEntry).not.toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-error')?.textContent).toContain(shown);
  });

  it('does nothing when the picker is cancelled', async () => {
    mocks.importShelf.mockResolvedValue({ cancelled: true });
    const host = await mountPage();

    button(host, '.mobile-shelves-import').click();
    await flush();

    expect(mocks.addShelfEntry).not.toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-error')).toBeNull();
  });
});

describe('the local shelf warning', () => {
  it('says local shelves are deleted with the app', async () => {
    const host = await mountPage();

    expect(host.querySelector('.mobile-shelves-local-warning')?.textContent).toContain('deleted with the app');
  });

  it('is not shown without a local shelf', async () => {
    mocks.loadShelfEntries.mockResolvedValue({ entries: [server], activeEntryID: 'e2' });
    const host = await mountPage();

    expect(host.querySelector('.mobile-shelves-local-warning')).toBeNull();
    expect(host.querySelector('.mobile-shelves-restore')).toBeNull();
  });
});

describe('restoring after a reinstall', () => {
  beforeEach(() => {
    mocks.loadShelfEntries.mockResolvedValue({ entries: [], activeEntryID: '' });
    mocks.listShelves.mockResolvedValue({ shelves: [] });
  });

  it('opens the restored zip as the shelf in use', async () => {
    const host = await mountPage();

    button(host, '.mobile-shelves-restore-zip').click();
    await flush();

    expect(mocks.upsertShelfEntry).toHaveBeenCalledWith({
      id: 'new-entry',
      type: 'local',
      name: 'Backup',
      localShelfId: 'local-2'
    });
    expect(mocks.reloadIntoApp).toHaveBeenCalled();
  });

  it('shows why a zip was rejected and stays on the prompt', async () => {
    mocks.importShelf.mockRejectedValue(new Error('not a PlainShelf shelf: the zip has no books/ directory'));
    const host = await mountPage();

    button(host, '.mobile-shelves-restore-zip').click();
    await flush();

    expect(mocks.upsertShelfEntry).not.toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-first-run .mobile-shelves-error')?.textContent).toContain('no books/ folder');
    expect(host.querySelector('.mobile-shelves-restore')).not.toBeNull();
  });

  it('stays quiet when the picker is cancelled', async () => {
    mocks.importShelf.mockResolvedValue({ cancelled: true });
    const host = await mountPage();

    button(host, '.mobile-shelves-restore-zip').click();
    await flush();

    expect(mocks.upsertShelfEntry).not.toHaveBeenCalled();
    expect(mocks.reloadIntoApp).not.toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-restore')).not.toBeNull();
  });

  it('records a skip and hides the prompt', async () => {
    const host = await mountPage();

    button(host, '.mobile-shelves-restore-skip').click();
    await flush();

    expect(mocks.dismissRestorePrompt).toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-restore')).toBeNull();
  });

  it('is not offered again once skipped', async () => {
    mocks.isRestorePromptDismissed.mockResolvedValue(true);
    const host = await mountPage();

    expect(host.querySelector('.mobile-shelves-restore')).toBeNull();
  });

  it('lists shelves that came back without their entries, as after a device transfer', async () => {
    mocks.listShelves.mockResolvedValue({
      shelves: [
        { id: 'local-1', name: 'Phone' },
        { id: 'local-3', name: 'Notes' }
      ]
    });
    const host = await mountPage();

    expect(mocks.addShelfEntry.mock.calls.map(([entry]) => [entry.name, entry.localShelfId])).toEqual([
      ['Phone', 'local-1'],
      ['Notes', 'local-3']
    ]);
    expect(mocks.setActiveShelfEntry).toHaveBeenCalledWith('new-entry');
    expect(mocks.reloadIntoApp).toHaveBeenCalled();
    expect(host.querySelector('.mobile-shelves-restore')).toBeNull();
  });

  it('shows no local shelf warning on an empty device', async () => {
    const host = await mountPage();

    expect(host.querySelector('.mobile-shelves-restore')).not.toBeNull();
    expect(host.querySelector('.mobile-shelves-local-warning')).toBeNull();
  });
});
