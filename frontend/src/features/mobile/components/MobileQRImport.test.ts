// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount as mountComponent } from '../../../../testing/mount';

const mocks = vi.hoisted(() => ({
  scan: vi.fn(), parse: vi.fn(), check: vi.fn(), save: vi.fn(),
  available: vi.fn(() => true), entries: [] as Array<Record<string, string>>
}));
vi.mock('@capacitor/core', () => ({
  Capacitor: { isNativePlatform: () => true, isPluginAvailable: mocks.available },
  registerPlugin: () => ({ scan: mocks.scan })
}));
vi.mock('@/providers/localShelf', () => ({ LocalShelf: { parseLANShareQR: mocks.parse, checkSource: mocks.check } }));
vi.mock('@/providers/mobileConfig', () => ({
  getShelfEntries: () => mocks.entries,
  newShelfEntry: () => ({ id: 'new' }),
  upsertShelfEntry: mocks.save
}));
vi.mock('@/features/mobile/utils/reloadIntoApp', () => ({ reloadIntoApp: vi.fn() }));
import MobileQRImport from './MobileQRImport.vue';
import { setLocale } from '@/i18n';

const one = { id: 'one', name: 'One', url: 'http://192.168.1.20/dav/one/' };
afterEach(() => vi.restoreAllMocks());
async function flush(): Promise<void> { await new Promise((r) => setTimeout(r)); }
function button(host: HTMLElement, label: string): HTMLButtonElement {
  const found = [...host.querySelectorAll('button')].find((b) => b.textContent?.trim() === label);
  if (!found) throw new Error(`missing ${label}`);
  return found;
}
async function scanned(): Promise<HTMLElement> {
  const { host } = mountComponent(MobileQRImport);
  button(host, 'Scan a LAN share QR code').click();
  await flush();
  return host;
}
beforeEach(() => {
  vi.clearAllMocks();
  setLocale('en');
  mocks.available.mockReturnValue(true);
  mocks.entries = [];
  mocks.scan.mockResolvedValue({ cancelled: false, text: 'private JSON' });
  mocks.parse.mockResolvedValue({ token: 'secret-token', shelves: [one] });
  mocks.check.mockResolvedValue({ books: 2 });
  mocks.save.mockResolvedValue(undefined);
});

describe('Android QR import', () => {
  it('previews without connecting or storing, then checks and saves secret separately on confirmation', async () => {
    const host = await scanned();
    expect(mocks.parse).toHaveBeenCalledWith({ text: 'private JSON' });
    expect(host.textContent).toContain(one.url);
    expect(host.textContent).not.toContain('secret-token');
    expect(mocks.check).not.toHaveBeenCalled();
    expect(mocks.save).not.toHaveBeenCalled();
    button(host, 'Import shelves').click(); await flush();
    expect(mocks.check).toHaveBeenCalledWith({ source: 'plainshelf', url: one.url, username: '', secret: 'secret-token' });
    expect(mocks.save).toHaveBeenCalledWith({ id: 'new', type: 'dav', name: 'One', davSource: 'plainshelf', davUrl: one.url }, 'secret-token');
    expect(host.textContent).toContain('Added');
    expect(host.textContent).not.toContain('secret-token');
  });
  it('cancels scanning without parsing or saving', async () => {
    mocks.scan.mockResolvedValue({ cancelled: true });
    await scanned();
    expect(mocks.parse).not.toHaveBeenCalled();
    expect(mocks.save).not.toHaveBeenCalled();
  });
  it('discards a cancelled preview', async () => {
    const host = await scanned();
    button(host, 'Cancel').click(); await flush();
    expect(host.textContent).not.toContain(one.url);
    expect(mocks.save).not.toHaveBeenCalled();
    expect(mocks.check).not.toHaveBeenCalled();
  });
  it.each(['unsupported QR version (expected v:1)', 'QR token is missing or invalid', 'QR url must be an HTTP(S) URL'])('shows shared parser rejection: %s', async (reason) => {
    mocks.parse.mockRejectedValue(new Error(reason));
    const host = await scanned();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain(reason);
    expect(mocks.save).not.toHaveBeenCalled();
    expect(mocks.check).not.toHaveBeenCalled();
  });
  it('handles missing native capability and scanner failure without exposing exception data', async () => {
    const logs = ['log', 'warn', 'error'].map((method) =>
      vi.spyOn(console, method as 'log' | 'warn' | 'error').mockImplementation(() => {})
    );
    mocks.available.mockReturnValue(false);
    const host = await scanned();
    expect(mocks.scan).not.toHaveBeenCalled();
    expect(host.textContent).toContain('unavailable');
    mocks.available.mockReturnValue(true);
    mocks.scan.mockRejectedValue(new Error('secret-token'));
    button(host, 'Scan a LAN share QR code').click(); await flush();
    expect(host.textContent).toContain('Could not scan');
    expect(host.textContent).not.toContain('secret-token');
    expect(mocks.save).not.toHaveBeenCalled();
    for (const log of logs) expect(log).not.toHaveBeenCalled();
  });
  it('skips existing URLs without replacing credentials and continues after individual failures', async () => {
    mocks.entries = [{ type: 'dav', davSource: 'plainshelf', davUrl: one.url.slice(0, -1) }];
    mocks.parse.mockResolvedValue({ token: 'secret-token', shelves: [one, { ...one, id: 'two', name: 'Two', url: one.url + 'two/' }, { ...one, id: 'three', name: 'Three', url: one.url + 'three/' }] });
    mocks.check.mockResolvedValueOnce({ code: 'SOURCE_UNAUTHORIZED', books: 0 }).mockResolvedValueOnce({ books: 3 });
    const host = await scanned();
    button(host, 'Import shelves').click(); await flush();
    expect(mocks.check).toHaveBeenCalledTimes(2);
    expect(mocks.save).toHaveBeenCalledTimes(1);
    expect(host.textContent).toContain('Already exists; skipped');
    expect(host.textContent).toContain('Wrong token');
    expect(host.textContent).toContain('Added');
  });
  it('does not save a failed check or expose an exception containing the token', async () => {
    mocks.check.mockRejectedValue(new Error('secret-token'));
    const host = await scanned();
    button(host, 'Import shelves').click(); await flush();
    expect(mocks.save).not.toHaveBeenCalled();
    expect(host.textContent).toContain('Could not import');
    expect(host.textContent).not.toContain('secret-token');
  });
  it('drops a scan answering after the form is closed', async () => {
    let resolve!: (value: { cancelled: boolean; text: string }) => void;
    mocks.scan.mockReturnValue(new Promise((r) => { resolve = r; }));
    const mounted = mountComponent(MobileQRImport);
    button(mounted.host, 'Scan a LAN share QR code').click();
    mounted.unmount();
    resolve({ cancelled: false, text: 'private JSON' }); await flush();
    expect(mocks.parse).not.toHaveBeenCalled();
    expect(mocks.save).not.toHaveBeenCalled();
  });
});
