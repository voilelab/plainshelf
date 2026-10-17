import { beforeEach, describe, expect, it, vi } from 'vitest';

const { plugin } = vi.hoisted(() => ({
  plugin: {
    beginBody: vi.fn(),
    appendBody: vi.fn(),
    request: vi.fn()
  }
}));

vi.mock('@capacitor/core', () => ({ registerPlugin: () => plugin }));

const { LOCAL_API_BASE, localShelfFetch } = await import('./localShelf');

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
});
