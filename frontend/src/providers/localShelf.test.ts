import { beforeEach, describe, expect, it, vi } from 'vitest';

const { plugin } = vi.hoisted(() => ({
  plugin: {
    beginBody: vi.fn(),
    appendBody: vi.fn(),
    discardBody: vi.fn(),
    request: vi.fn()
  }
}));

vi.mock('@capacitor/core', () => ({ registerPlugin: () => plugin }));

const { LOCAL_API_BASE, localShelfErrorKey, localShelfExportFileName, localShelfFetch } = await import('./localShelf');

const fetchMock = vi.fn();
let staged: Uint8Array[] = [];

beforeEach(() => {
  staged = [];
  fetchMock.mockReset().mockResolvedValue(new Response('from fetch'));
  vi.stubGlobal('fetch', fetchMock);
  plugin.beginBody.mockReset().mockResolvedValue({ bodyId: 'b1' });
  plugin.appendBody.mockReset().mockImplementation(async ({ data }: { data: string }) => {
    staged.push(Uint8Array.from(atob(data), (c) => c.charCodeAt(0)));
  });
  plugin.discardBody.mockReset().mockResolvedValue(undefined);
  plugin.request.mockReset().mockResolvedValue({
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    body: btoa('{"ok":true}')
  });
});

describe('localShelfFetch', () => {
  it('leaves reads to the WebView, which the native client intercepts', async () => {
    const res = await localShelfFetch(`${LOCAL_API_BASE}/api/shelves/s/books`);
    expect(await res.text()).toBe('from fetch');
    expect(plugin.request).not.toHaveBeenCalled();
  });

  it('leaves a request outside the local shelf alone', async () => {
    await localShelfFetch('http://192.168.1.10:20000/api/shelves', { method: 'POST', body: '{}' });
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(plugin.request).not.toHaveBeenCalled();
  });

  it('stages a large write body in bounded chunks and sends the path without the prefix', async () => {
    const body = new Uint8Array(2.5 * (1 << 20)).map((_, i) => i % 251);

    const res = await localShelfFetch(`${LOCAL_API_BASE}/api/shelves/s/books/b/sources/x/content?a=1`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'text/plain' },
      body
    });

    expect(plugin.beginBody).toHaveBeenCalledOnce();
    expect(plugin.appendBody).toHaveBeenCalledTimes(3);
    expect(staged.map((c) => c.length)).toEqual([1 << 20, 1 << 20, 0.5 * (1 << 20)]);
    // Compared as one buffer: toEqual walks 2.5 MiB element by element.
    expect(Buffer.concat(staged).equals(Buffer.from(body))).toBe(true);
    expect(plugin.request).toHaveBeenCalledWith({
      method: 'PATCH',
      path: '/api/shelves/s/books/b/sources/x/content?a=1',
      headers: { 'content-type': 'text/plain' },
      bodyId: 'b1'
    });
    expect(await res.json()).toEqual({ ok: true });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('sends a write without a body without staging one', async () => {
    plugin.request.mockResolvedValue({ status: 204, headers: {}, body: '' });

    const res = await localShelfFetch(`${LOCAL_API_BASE}/api/shelves/s/books/b`, { method: 'DELETE' });

    expect(plugin.beginBody).not.toHaveBeenCalled();
    expect(plugin.request).toHaveBeenCalledWith(expect.objectContaining({ bodyId: undefined }));
    expect(res.status).toBe(204);
  });

  it('discards a staged body when the upload is cut short', async () => {
    const controller = new AbortController();
    plugin.appendBody.mockImplementationOnce(async () => controller.abort());

    await expect(
      localShelfFetch(`${LOCAL_API_BASE}/api/shelves/s/books/import`, {
        method: 'POST',
        body: new Uint8Array(3 * (1 << 20)),
        signal: controller.signal
      })
    ).rejects.toThrow();

    expect(plugin.discardBody).toHaveBeenCalledWith({ bodyId: 'b1' });
    expect(plugin.request).not.toHaveBeenCalled();
  });
});

describe('localShelfExportFileName', () => {
  const day = new Date(2026, 9, 7);

  it('names the backup after the shelf and the date', () => {
    expect(localShelfExportFileName('Phone', day)).toBe('plainshelf-Phone-20261007.zip');
  });

  it('replaces characters a file name cannot hold', () => {
    expect(localShelfExportFileName(' a/b:c? ', day)).toBe('plainshelf-a-b-c-20261007.zip');
  });

  it('drops an empty name', () => {
    expect(localShelfExportFileName('  ', day)).toBe('plainshelf-20261007.zip');
  });
});

describe('localShelfErrorKey', () => {
  // Wording as the Go sentinels produce it, prefixed the way util.Errorf wraps them.
  it.each([
    ['plainshelf: background tasks are still running', 'mobileShelves.errorBusy'],
    ['not a PlainShelf shelf: the zip has no books/ directory', 'mobileShelves.errorNotShelf'],
    ['internal/shelfzip.plan: zip entry escapes the shelf: "../evil"', 'mobileShelves.errorUnsafePath'],
    ['zip unpacks to more than 512 MiB', 'mobileShelves.errorTooLarge'],
    ['mobile.ImportShelf.func1: zip: not a valid zip file', 'mobileShelves.errorNotZip']
  ])('explains %j', (message, key) => {
    expect(localShelfErrorKey(new Error(message))).toBe(key);
  });

  it('leaves anything else to the raw message', () => {
    expect(localShelfErrorKey(new Error('disk full'))).toBeNull();
  });
});
