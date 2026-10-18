// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { plugin, localFetch } = vi.hoisted(() => ({
  plugin: {
    openPCloudShelf: vi.fn(),
    openSourceShelf: vi.fn(),
    setShelfShowNsfw: vi.fn(),
    nsfwMarks: vi.fn()
  },
  localFetch: vi.fn()
}));

vi.mock('./localShelf', () => ({
  LOCAL_API_BASE: 'https://localhost/__plainshelf_local',
  LocalShelf: plugin,
  localShelfFetch: localFetch
}));

type Module = typeof import('./embeddedPCloud');

const entry = {
  id: 'entry-1',
  type: 'pcloud' as const,
  name: 'Cloud',
  pcloudClientId: 'key',
  pcloudHost: 'eapi.pcloud.com',
  pcloudShelfRoot: '/PlainShelf/Shelf',
  pcloudUserId: '42'
};
const base = 'pcloud://eapi.pcloud.com/42';
const shelfPrefix = `${base}/api/shelves/${encodeURIComponent(entry.pcloudShelfRoot)}`;

const fetchMock = vi.fn();
let mod: Module;
let setShowNsfwOnDevice: (value: boolean) => void;
let currentCacheScopeKey: () => string;

beforeEach(async () => {
  // Fresh modules per test: what Go was last given is module state, and the
  // client state has to be the instance the module under test reads.
  vi.resetModules();
  mod = await import('./embeddedPCloud');
  const { setActiveShelfID, setApiBase } = await import('@/api/client');
  ({ setShowNsfwOnDevice } = await import('@/composables/useDeviceNsfwPreference'));
  ({ currentCacheScopeKey } = await import('./cacheScope'));
  setApiBase(base);
  setActiveShelfID(entry.pcloudShelfRoot);
  setShowNsfwOnDevice(false);
  plugin.openPCloudShelf.mockReset().mockResolvedValue(undefined);
  plugin.openSourceShelf.mockReset().mockResolvedValue(undefined);
  plugin.setShelfShowNsfw.mockReset().mockResolvedValue(undefined);
  plugin.nsfwMarks.mockReset();
  localFetch.mockReset().mockResolvedValue(new Response('from go'));
  fetchMock.mockReset().mockResolvedValue(new Response('from fetch'));
  vi.stubGlobal('fetch', fetchMock);
});

describe('openEmbeddedPCloudShelf', () => {
  it('opens the entry under its own cache scope, with no token', async () => {
    const scope = currentCacheScopeKey();
    await mod.openEmbeddedPCloudShelf(entry);

    expect(plugin.openPCloudShelf).toHaveBeenCalledWith({
      id: 'pcloud',
      entryId: 'entry-1',
      name: 'Cloud',
      host: 'eapi.pcloud.com',
      folder: '/PlainShelf/Shelf',
      scope,
      showNsfw: false
    });
    // The identity downloads and progress are filed under is untouched.
    expect(currentCacheScopeKey()).toBe(scope);
  });

  it('opens a dav entry through Go with no secret on the bridge', async () => {
    await mod.openEmbeddedPCloudShelf({
      id: 'entry-3',
      type: 'dav',
      name: 'NAS',
      davSource: 'webdav',
      davUrl: 'https://nas.example/dav/books/',
      davUsername: 'reader'
    });

    expect(plugin.openSourceShelf).toHaveBeenCalledWith({
      id: 'pcloud',
      entryId: 'entry-3',
      name: 'NAS',
      source: 'webdav',
      url: 'https://nas.example/dav/books/',
      username: 'reader',
      showNsfw: false
    });
    expect(plugin.openPCloudShelf).not.toHaveBeenCalled();
  });
});

describe('embeddedPCloudFetch', () => {
  it.each(['string', 'URL', 'Request'] as const)('sends the server mode query to Go (%s), without opening the shelf', async (kind) => {
    const url = `${base}/api/mode`;
    const input = kind === 'Request' ? new Request(url) : kind === 'URL' ? new URL(url) : url;
    const init = { signal: new AbortController().signal };
    localFetch.mockResolvedValueOnce(Response.json({ read_only: false }));

    const res = await mod.embeddedPCloudFetch(input, init);

    expect(await res.json()).toEqual({ read_only: false });
    const [target, options] = localFetch.mock.calls[0]!;
    expect(target instanceof Request ? target.url : target).toBe('https://localhost/__plainshelf_local/api/mode');
    expect(options).toBe(init);
    expect(plugin.openPCloudShelf).not.toHaveBeenCalled();
    expect(plugin.setShelfShowNsfw).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('sends the entry shelf routes to the Go shelf', async () => {
    await mod.openEmbeddedPCloudShelf(entry);
    const res = await mod.embeddedPCloudFetch(`${shelfPrefix}/books/b1/content`);

    expect(await res.text()).toBe('from go');
    expect(localFetch).toHaveBeenCalledWith(
      'https://localhost/__plainshelf_local/api/shelves/pcloud/books/b1/content',
      undefined
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it.each([
    `${base}/api/setting/show_nsfw`,
    `${base}/api/mode/other`,
    `${base}/api/modes`,
    'https://other.example/api/mode'
  ])('leaves other URLs alone: %s', async (url) => {
    await mod.openEmbeddedPCloudShelf(entry);
    await mod.embeddedPCloudFetch(url);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith(url, undefined);
    expect(localFetch).not.toHaveBeenCalled();
  });

  it('fails like no network when the shelf was never opened, or failed to', async () => {
    await expect(mod.embeddedPCloudFetch(`${shelfPrefix}/books`)).rejects.toBeInstanceOf(TypeError);

    plugin.openPCloudShelf.mockRejectedValue(new Error('no pCloud token is stored'));
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    expect(await mod.openEmbeddedPCloudShelf(entry)).toBe(false);
    await expect(mod.embeddedPCloudFetch(`${shelfPrefix}/books`)).rejects.toBeInstanceOf(TypeError);
    expect(localFetch).not.toHaveBeenCalled();
  });

  it('waits for an open in progress, but no longer than the request', async () => {
    let finish: () => void = () => undefined;
    plugin.openPCloudShelf.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)));
    void mod.openEmbeddedPCloudShelf(entry);

    const controller = new AbortController();
    const aborted = mod.embeddedPCloudFetch(`${shelfPrefix}/books`, { signal: controller.signal });
    controller.abort();
    await expect(aborted).rejects.toMatchObject({ name: 'AbortError' });

    const waiting = mod.embeddedPCloudFetch(`${shelfPrefix}/books`);
    finish();
    expect(await (await waiting).text()).toBe('from go');
  });

  it('hands Go a changed device choice before the request, once', async () => {
    await mod.openEmbeddedPCloudShelf(entry);
    setShowNsfwOnDevice(true);
    await mod.embeddedPCloudFetch(`${shelfPrefix}/books`);
    await mod.embeddedPCloudFetch(`${shelfPrefix}/books`);

    expect(plugin.setShelfShowNsfw).toHaveBeenCalledTimes(1);
    expect(plugin.setShelfShowNsfw).toHaveBeenCalledWith({ id: 'pcloud', show: true });
    expect(plugin.setShelfShowNsfw.mock.invocationCallOrder[0]).toBeLessThan(
      localFetch.mock.invocationCallOrder[0]
    );
  });

  it('pushes again when the choice changes while a push is on the bridge', async () => {
    await mod.openEmbeddedPCloudShelf(entry);
    let release: () => void = () => undefined;
    plugin.setShelfShowNsfw.mockImplementationOnce(() => new Promise<void>((resolve) => (release = resolve)));
    setShowNsfwOnDevice(true);
    const request = mod.embeddedPCloudFetch(`${shelfPrefix}/books`);
    await vi.waitFor(() => expect(plugin.setShelfShowNsfw).toHaveBeenCalledTimes(1));

    setShowNsfwOnDevice(false);
    release();
    await request;

    expect(plugin.setShelfShowNsfw.mock.calls.map(([options]) => options.show)).toEqual([true, false]);
    expect(plugin.setShelfShowNsfw.mock.invocationCallOrder[1]).toBeLessThan(
      localFetch.mock.invocationCallOrder[0]
    );
  });

  it('refuses the request when Go cannot be told', async () => {
    setShowNsfwOnDevice(true);
    await mod.openEmbeddedPCloudShelf(entry);
    setShowNsfwOnDevice(false);
    plugin.setShelfShowNsfw.mockRejectedValue(new Error('bridge down'));

    await expect(mod.embeddedPCloudFetch(`${shelfPrefix}/books`)).rejects.toThrow('bridge down');
    expect(localFetch).not.toHaveBeenCalled();
  });
});

describe('EmbeddedPCloudReader', () => {
  it('keeps the adult-content choice on the device', () => {
    expect(new mod.EmbeddedPCloudReader().filtersNsfwOnDevice()).toBe(true);
  });

  it('reads every mark from Go, and none when Go cannot answer', async () => {
    plugin.nsfwMarks.mockResolvedValueOnce({ marks: { b1: { nsfw: true }, b2: { nsfw: false } } });
    const reader = new mod.EmbeddedPCloudReader();

    const marks = await reader.localNsfwMarks();
    expect(marks?.get('b1')).toEqual({ nsfw: true });
    expect(marks?.size).toBe(2);

    plugin.nsfwMarks.mockRejectedValueOnce(new Error('not open'));
    expect(await reader.localNsfwMarks()).toBeNull();
  });
});

describe('Go offline downloads', () => {
  it('uses the engine for capabilities, state, download, listing and removal, while keeping library writes disabled', async () => {
    const { setApiTransport } = await import('@/api/client');
    window.__PLAINSHELF_READ_ONLY__ = true;
    const { isWritableProvider } = await import('./index');
    setApiTransport({ fetch: mod.embeddedPCloudFetch, allowMobileWrites: false });
    await mod.openEmbeddedPCloudShelf(entry);
    const reader = new mod.EmbeddedPCloudReader();
    expect(isWritableProvider(reader)).toBe(false);
    const book = { meta: { id: 'b', title: 'Book', authors: [], language: 'en', format: 'txt' }, folder: [], download_state: 'downloaded' };
    localFetch.mockResolvedValueOnce(Response.json(book));
    expect(await reader.getDownloadState('b')).toBe('downloaded');
    localFetch.mockResolvedValueOnce(Response.json({ book }));
    await reader.downloadBook('b');
    expect(localFetch.mock.lastCall?.[0]).toBe('https://localhost/__plainshelf_local/api/shelves/pcloud/books/b/download');
    expect(localFetch.mock.lastCall?.[1].method).toBe('PUT');
    localFetch.mockResolvedValueOnce(Response.json({ books: [{ book, size_bytes: 123, downloaded_at: '2026-10-18T00:00:00Z' }, { book: { ...book, meta: { ...book.meta, id: 'adult', nsfw: true } }, size_bytes: 456, downloaded_at: '2026-10-18T00:00:00Z' }] }));
    expect(await reader.listDownloadedBookEntries()).toMatchObject([{ book: { id: 'b', download_state: 'downloaded' }, sizeBytes: 123, downloadedAt: '2026-10-18T00:00:00Z' }]);
    localFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await reader.removeDownload('b');
    expect(localFetch.mock.lastCall?.[1].method).toBe('DELETE');
    const { updateBook } = await import('@/api/books');
    await expect(updateBook('b', { title: 'Changed' })).rejects.toThrow('read-only');
    window.__PLAINSHELF_READ_ONLY__ = false;
  });
});
