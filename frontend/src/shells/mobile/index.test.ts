import { beforeEach, describe, expect, it, vi } from 'vitest';

const h = vi.hoisted(() => ({
  entry: null as Record<string, unknown> | null,
  token: '',
  openShelf: vi.fn(),
  setApiTransport: vi.fn(),
  registerShell: vi.fn(),
  mobileProvider: vi.fn()
}));

vi.mock('@/api/client', () => ({ setApiTransport: h.setApiTransport }));
vi.mock('@/providers/mobileConfig', () => ({
  initMobileConfig: vi.fn(),
  getActiveShelfEntry: () => h.entry,
  getActiveShelfEntryToken: () => h.token,
  isShelfEntryConfigured: (entry: Record<string, unknown> | null) =>
    entry?.type !== 'pcloud' || Boolean(entry.pcloudHost && entry.pcloudShelfRoot)
}));
vi.mock('@/providers/embeddedPCloud', () => ({
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
  return h.mobileProvider.mock.calls[0][0];
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
  });

  it('does not wait for the shelf to open', async () => {
    h.entry = pcloudEntry;
    h.token = 'secret';
    // Never awaited, and settles false rather than rejecting.
    h.openShelf.mockReturnValue(new Promise(() => undefined));
    await installMobileShell();

    expect(h.registerShell).toHaveBeenCalledTimes(1);
    expect((providerRemote() as object).constructor.name).toBe('EmbeddedPCloudReader');
  });

  it('leaves a pCloud entry without its token to the setup form', async () => {
    h.entry = pcloudEntry;
    await installMobileShell();

    expect(h.openShelf).not.toHaveBeenCalled();
    expect((providerRemote() as object).constructor.name).toBe('ServerBookshelfProvider');
  });
});
