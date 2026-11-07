// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  checkSource: vi.fn(),
  upsertShelfEntry: vi.fn(),
  listServerShelves: vi.fn()
}));

vi.mock('@capacitor/core', () => ({ Capacitor: { isNativePlatform: () => true }, registerPlugin: () => ({}) }));
vi.mock('@capacitor/browser', () => ({ Browser: { open: vi.fn(), close: vi.fn() } }));
vi.mock('@/providers/localShelf', () => ({ LocalShelf: { checkSource: mocks.checkSource } }));
vi.mock('@/providers/mobileConfig', () => ({
  getShelfEntryToken: vi.fn(() => Promise.resolve('')),
  newShelfEntry: (type: string) => ({ id: 'new-entry', type, name: '' }),
  upsertShelfEntry: mocks.upsertShelfEntry
}));
vi.mock('@/api/shelves', () => ({ listServerShelves: mocks.listServerShelves }));
vi.mock('@/features/mobile/utils/reloadIntoApp', () => ({ reloadIntoApp: vi.fn() }));

import MobileShelfForm from './MobileShelfForm.vue';
import { setLocale } from '@/i18n';

let mounted: { app: App; host: HTMLElement } | null = null;

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

function mountForm(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(MobileShelfForm, { entry: null }) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

function type(input: HTMLInputElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

function button(host: HTMLElement, text: string): HTMLButtonElement {
  const found = [...host.querySelectorAll('button')].find((b) => b.textContent?.trim() === text);
  if (!found) {
    throw new Error(`no button ${text}`);
  }
  return found;
}

function selectMode(host: HTMLElement, label: string): void {
  const found = [...host.querySelectorAll<HTMLElement>('.shelf-source-tile, .shelf-source-shortcut')].find(
    (tile) => tile.querySelector('strong')?.textContent?.trim() === label
  );
  if (!found) {
    throw new Error(`no mode ${label}`);
  }
  found.click();
}

async function openDavForm(host: HTMLElement): Promise<{ url: HTMLInputElement; secret: HTMLInputElement }> {
  selectMode(host, 'Another device');
  await flush();
  type(host.querySelector('input[type="text"]') as HTMLInputElement, 'Living room');
  const url = host.querySelector('input[type="url"]') as HTMLInputElement;
  const secret = host.querySelector('input[type="password"]') as HTMLInputElement;
  type(url, 'http://192.168.1.20:20001/dav/a/');
  type(secret, 'tok');
  await flush();
  return { url, secret };
}

beforeEach(() => {
  setLocale('en');
  mocks.checkSource.mockReset();
  mocks.upsertShelfEntry.mockReset().mockResolvedValue(undefined);
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('MobileShelfForm dav entries', () => {
  it('checks the values on save and saves them when the shelf answers', async () => {
    mocks.checkSource.mockResolvedValue({ books: 2 });
    const host = mountForm();
    await openDavForm(host);

    button(host, 'Add shelf').click();
    await flush();

    expect(mocks.checkSource).toHaveBeenCalledWith({
      source: 'plainshelf',
      url: 'http://192.168.1.20:20001/dav/a/',
      username: '',
      secret: 'tok'
    });
    expect(mocks.upsertShelfEntry).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'dav', davUrl: 'http://192.168.1.20:20001/dav/a/' }),
      'tok'
    );
  });

  it('saves nothing when the check on save fails', async () => {
    mocks.checkSource.mockResolvedValue({ code: 'SOURCE_UNAUTHORIZED', books: 0 });
    const host = mountForm();
    await openDavForm(host);

    button(host, 'Add shelf').click();
    await flush();

    expect(mocks.upsertShelfEntry).not.toHaveBeenCalled();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Wrong token');
  });

  it('says a refused token apart from no connection', async () => {
    mocks.checkSource.mockResolvedValueOnce({ code: 'SOURCE_UNAUTHORIZED', books: 0 });
    const host = mountForm();
    await openDavForm(host);
    button(host, 'Add shelf').click();
    await flush();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Wrong token');

    mocks.checkSource.mockResolvedValueOnce({ code: 'SOURCE_UNAVAILABLE', books: 0 });
    button(host, 'Add shelf').click();
    await flush();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Cannot connect');
  });
});

describe('loading the shelves of a server', () => {
  it('asks that server with the typed token, leaving the active client alone', async () => {
    mocks.listServerShelves.mockReset().mockResolvedValue([{ id: 'main', name: 'Main', readOnly: false }]);
    const host = mountForm();
    await flush();
    selectMode(host, 'PlainShelf server');
    await flush();

    type(host.querySelector('input[type="url"]') as HTMLInputElement, 'http://192.168.1.10:20000');
    type(host.querySelector('input[type="password"]') as HTMLInputElement, 'probe');
    await flush();
    button(host, 'Load library').click();
    await flush();

    expect(mocks.listServerShelves).toHaveBeenCalledWith({ apiBase: 'http://192.168.1.10:20000', token: 'probe' });
  });
});

describe('the QR mode', () => {
  it('shows only the scanner, without the manual form', async () => {
    const host = mountForm();
    await flush();

    selectMode(host, 'Scan a QR code to add them all');
    await flush();

    expect(host.querySelector('form')).toBeNull();
    expect(button(host, 'Scan a LAN share QR code')).toBeTruthy();
  });
});

describe('choosing a source', () => {
  it('starts on the picker, grouped by where the shelf lives', async () => {
    const host = mountForm();
    await flush();

    expect(host.querySelector('form')).toBeNull();
    const groups = [...host.querySelectorAll('.shelf-source-group-label')].map((label) => label.textContent?.trim());
    expect(groups).toEqual(['On this device', 'Read from elsewhere (read-only)']);
    const tiles = [...host.querySelectorAll('.shelf-source-tile strong')].map((label) => label.textContent?.trim());
    expect(tiles).toEqual(['Local shelf', 'Another device', 'PlainShelf server', 'pCloud']);
  });

  it('goes back to the picker from the chosen source', async () => {
    const host = mountForm();
    await flush();
    selectMode(host, 'pCloud');
    await flush();

    button(host, 'Change').click();
    await flush();

    expect(host.querySelector('.shelf-source-tile')).not.toBeNull();
  });
});
