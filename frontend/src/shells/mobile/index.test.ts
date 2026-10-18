import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({
  entry: null as Record<string, unknown> | null,
  token: '',
  openShelf: vi.fn(),
  setApiTransport: vi.fn(),
  registerShell: vi.fn(),
  listServerShelves: vi.fn(),
  mobileProvider: vi.fn()
}));

vi.mock('@/api/shelves', () => ({ listServerShelves: h.listServerShelves }));
vi.mock('@/api/client', () => ({ setApiTransport: h.setApiTransport }));
vi.mock('@/providers/mobileConfig', () => ({
  initMobileConfig: vi.fn(),
  getActiveShelfEntry: () => h.entry,
  getActiveShelfEntryToken: () => h.token,
  isShelfEntryConfigured: (entry: Record<string, unknown> | null) =>
    entry?.type !== 'pcloud' || Boolean(entry.pcloudHost && entry.pcloudShelfRoot)
}));
vi.mock('@/providers/embeddedPCloud', () => ({
  EMBEDDED_PCLOUD_SHELF_ID: 'pcloud',
  EmbeddedPCloudReader: class EmbeddedPCloudReader {},
  embeddedPCloudFetch: 'embeddedPCloudFetch',
  openEmbeddedPCloudShelf: h.openShelf
}));
vi.mock('@/providers/localShelf', () => ({ localShelfFetch: 'localShelfFetch' }));
vi.mock('@/providers/serverBookshelfProvider', () => ({ ServerBookshelfProvider: class ServerBookshelfProvider {} }));
vi.mock('@/providers/mobileBookshelfProvider', () => ({
  MobileBookshelfProvider: class {
    constructor(remote: unknown) {
      h.mobileProvider(remote);
    }
  }
}));
vi.mock('@/providers/filesystemMobileBookCache', () => ({ FilesystemMobileBookCache: class {} }));
vi.mock('@/providers/filesystemMobileCoverCache', () => ({ FilesystemMobileCoverCache: class {} }));
vi.mock('@/providers/shell', () => ({ registerShell: h.registerShell }));
vi.mock('./deviceDocumentStorage', () => ({ createMobileDeviceDocumentStorage: vi.fn() }));
vi.mock('./shelfInfo', () => ({ activeMobileShelfInfo: vi.fn() }));
vi.mock('./routerGuard', () => ({ installMobileRouterGuards: vi.fn() }));
vi.mock('./shelfPicker', () => ({ createMobileShelfPicker: vi.fn() }));

const { installMobileShell } = await import('./index');

const pcloudEntry = { id: 'e1', type: 'pcloud', pcloudHost: 'eapi.pcloud.com', pcloudShelfRoot: '/Shelf' };

/** The remote the registered shell's provider wraps. */
function providerRemote(): unknown {
  h.registerShell.mock.calls[0][0].createProvider();
  return h.mobileProvider.mock.calls[0]?.[0] ?? h.registerShell.mock.calls[0][0].createProvider();
}

beforeEach(() => {
  h.entry = null;
  h.token = '';
  for (const fn of [h.openShelf, h.setApiTransport, h.registerShell, h.mobileProvider]) {
    fn.mockReset();
  }
  h.openShelf.mockResolvedValue(true);
});

describe('installMobileShell', () => {
  it('reads a pCloud entry through the embedded Go server, read-only', async () => {
    h.entry = pcloudEntry;
    h.token = 'secret';
    await installMobileShell();

    expect(h.openShelf).toHaveBeenCalledWith(pcloudEntry);
    expect(h.setApiTransport).toHaveBeenCalledWith({ fetch: 'embeddedPCloudFetch', allowMobileWrites: false });
    expect((providerRemote() as object).constructor.name).toBe('EmbeddedPCloudReader');
    expect(h.mobileProvider).not.toHaveBeenCalled();
  });

  it('does not wait for the shelf to open', async () => {
    h.entry = pcloudEntry;
    h.token = 'secret';
    // Never awaited, and settles false rather than rejecting.
    h.openShelf.mockReturnValue(new Promise(() => undefined));
    await installMobileShell();

    expect(h.registerShell).toHaveBeenCalledTimes(1);
    expect((providerRemote() as object).constructor.name).toBe('EmbeddedPCloudReader');
    expect(h.mobileProvider).not.toHaveBeenCalled();
  });

  it('leaves a pCloud entry without its token to the setup form', async () => {
    h.entry = pcloudEntry;
    await installMobileShell();

    expect(h.openShelf).not.toHaveBeenCalled();
    expect((providerRemote() as object).constructor.name).toBe('ServerBookshelfProvider');
  });
});

it('uses the Go reader directly for dav, while server entries keep the mobile cache', async () => {
  h.entry = { id: 'dav', type: 'dav', davSource: 'webdav', davUrl: 'https://nas.example/books/' };
  await installMobileShell();
  expect((providerRemote() as object).constructor.name).toBe('EmbeddedPCloudReader');
  expect(h.mobileProvider).not.toHaveBeenCalled();
  h.registerShell.mockClear();
  h.entry = { id: 'server', type: 'server' };
  await installMobileShell();
  expect((providerRemote() as object).constructor.name).toBe('ServerBookshelfProvider');
  expect(h.mobileProvider).toHaveBeenCalled();
});

vi.mock('./platform', () => ({ createMobilePlatform: () => ({}) }));


describe('mobile shelf capabilities', () => {
  it.each([undefined, { download: false, rescan: false, char_count: false, fingerprint: false }])(
    'keeps wrapper downloads while preserving the remote engine answer: %j', async (capabilities) => {
      h.entry = { id: 'e1', type: 'server' };
      h.listServerShelves.mockResolvedValue([{ id: 'main', name: 'Remote name', capabilities,
        offline: { snapshot_at: '2026-10-16T09:12:44Z' } }]);
      await installMobileShell();
      const shelf = await h.registerShell.mock.calls[0][0].loadActiveShelfInfo({ id: 'main', name: 'Device name' });
      expect(shelf.name).toBe('Device name');
      expect(shelf.capabilities).toEqual({ ...capabilities, download: true });
      expect(shelf.offline.snapshot_at).toBe('2026-10-16T09:12:44Z');
    }
  );
  it('keeps downloads reachable while a remote server is offline', async () => {
    h.entry = { id: 'e1', type: 'server' };
    h.listServerShelves.mockRejectedValue(new TypeError('offline'));
    await installMobileShell();
    const shelf = await h.registerShell.mock.calls[0][0].loadActiveShelfInfo({ id: 'main', name: 'Device name' });
    expect(shelf.capabilities.download).toBe(true);
  });
  it.each([true, false])('reads embedded download capability %s without replacing the device identity', async (download) => {
    h.entry = pcloudEntry;
    h.token = 'secret';
    h.listServerShelves.mockResolvedValue([{ id: 'local', name: 'Other shelf' }, { id: 'pcloud', name: 'Engine name', kind: 'virtual',
      capabilities: { char_count: false, fingerprint: false, rescan: true, download } }]);
    await installMobileShell();
    const shelf = await h.registerShell.mock.calls[0][0].loadActiveShelfInfo({ id: '/Shelf', name: 'Device name' });
    expect(shelf.id).toBe('/Shelf');
    expect(shelf.kind).toBe('virtual');
    expect(shelf.capabilities.char_count).toBe(false);
    expect(shelf.capabilities.download).toBe(download);
  });
});
