import { describe, expect, it, vi } from 'vitest';

import { PCloudClient } from './client';
import { PCloudError } from './errors';

const TOKEN = 'test-token';

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init
  });
}

function makeClient(fetchImpl: typeof fetch) {
  return new PCloudClient({
    host: 'api.pcloud.com',
    accessToken: TOKEN,
    fetchImpl,
    // Retry backoff would otherwise add seconds to the suite.
    sleepImpl: async () => {}
  });
}

describe('PCloudClient construction', () => {
  it('requires a host and a token', () => {
    const fetchImpl = vi.fn() as unknown as typeof fetch;
    expect(() => new PCloudClient({ host: '  ', accessToken: TOKEN, fetchImpl })).toThrow(PCloudError);
    expect(() => new PCloudClient({ host: 'api.pcloud.com', accessToken: ' ', fetchImpl })).toThrow(PCloudError);
  });

  it('accepts a host given with a scheme', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0, metadata: { name: 'x', isfolder: true } }));
    const client = new PCloudClient({
      host: 'https://eapi.pcloud.com/',
      accessToken: TOKEN,
      fetchImpl: fetchImpl as unknown as typeof fetch
    });

    await client.listFolder({ path: '/shelf' });

    expect(fetchImpl.mock.calls[0][0]).toContain('https://eapi.pcloud.com/listfolder');
  });
});

describe('PCloudClient.call', () => {
  it('sends the token as a Bearer header rather than in the URL', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0, metadata: { name: 'x', isfolder: true } }));
    await makeClient(fetchImpl as unknown as typeof fetch).listFolder({ path: '/shelf' });

    const [url, init] = fetchImpl.mock.calls[0];
    expect(String(url)).not.toContain(TOKEN);
    expect((init as RequestInit).headers).toMatchObject({ Authorization: `Bearer ${TOKEN}` });
  });

  it('turns a non-zero result into a PCloudError carrying the code', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 2005, error: 'Directory does not exist.' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder({ path: '/missing' })).rejects.toMatchObject(
      { name: 'PCloudError', result: 2005 }
    );
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('retries a 4xxx rate-limit result and then succeeds', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ result: 4000, error: 'Too many requests.' }))
      .mockResolvedValueOnce(jsonResponse({ result: 0, metadata: { name: 'shelf', isfolder: true } }));

    const meta = await makeClient(fetchImpl as unknown as typeof fetch).listFolder({ path: '/shelf' });

    expect(meta.name).toBe('shelf');
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it('does not retry a permanent error', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 1101, error: 'Not supported.' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder({ path: '/' })).rejects.toBeInstanceOf(
      PCloudError
    );
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('reports an HTTP failure with its status', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response('nope', { status: 401, statusText: 'Unauthorized' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder({ path: '/shelf' })).rejects.toMatchObject({
      name: 'PCloudError',
      status: 401
    });
  });
});

describe('PCloudClient.listFolderRecursive', () => {
  it('asks for the whole tree in one request', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0, metadata: { name: 'shelf', isfolder: true } }));
    await makeClient(fetchImpl as unknown as typeof fetch).listFolderRecursive({ path: '/shelf' });

    expect(fetchImpl).toHaveBeenCalledTimes(1);
    expect(String(fetchImpl.mock.calls[0][0])).toContain('recursive=1');
  });

  it('walks the tree manually when the root rejects recursive listing', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = new URL(String(input));
      const recursive = url.searchParams.get('recursive') === '1';
      const folderid = url.searchParams.get('folderid');

      if (recursive && folderid === '0') {
        return Promise.resolve(jsonResponse({ result: 1101, error: 'Not supported.' }));
      }
      if (folderid === '0') {
        return Promise.resolve(
          jsonResponse({
            result: 0,
            metadata: {
              name: '/',
              isfolder: true,
              folderid: 0,
              contents: [
                { name: 'shelf', isfolder: true, folderid: 7 },
                { name: 'readme.txt', isfolder: false, fileid: 9 }
              ]
            }
          })
        );
      }
      return Promise.resolve(
        jsonResponse({
          result: 0,
          metadata: {
            name: 'shelf',
            isfolder: true,
            folderid: 7,
            contents: [{ name: 'books', isfolder: true, folderid: 8, contents: [] }]
          }
        })
      );
    });

    const tree = await makeClient(fetchImpl as unknown as typeof fetch).listFolderRecursive({ folderid: 0 });

    expect(tree.contents?.map((item) => item.name)).toEqual(['shelf', 'readme.txt']);
    expect(tree.contents?.[0].contents?.[0].name).toBe('books');
  });
});

describe('PCloudClient downloads', () => {
  it('resolves a download URL from hosts and path', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(jsonResponse({ result: 0, hosts: ['c1.pcloud.com', 'c2.pcloud.com'], path: '/dl/abc' }));

    const url = await makeClient(fetchImpl as unknown as typeof fetch).getFileLink(42);

    expect(url).toBe('https://c1.pcloud.com/dl/abc');
  });

  it('fails clearly when getfilelink returns no host', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0, hosts: [], path: '/dl/abc' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).getFileLink(42)).rejects.toBeInstanceOf(PCloudError);
  });

  it('does not send the API token to the download host', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('getfilelink')) {
        return Promise.resolve(jsonResponse({ result: 0, hosts: ['c1.pcloud.com'], path: '/dl/abc' }));
      }
      return Promise.resolve(new Response('book text'));
    });

    const text = await makeClient(fetchImpl as unknown as typeof fetch).downloadText(42);

    expect(text).toBe('book text');
    const downloadInit = fetchImpl.mock.calls[1][1] as RequestInit | undefined;
    expect(downloadInit?.headers).toBeUndefined();
  });

  it('surfaces a failed download with its status', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('getfilelink')) {
        return Promise.resolve(jsonResponse({ result: 0, hosts: ['c1.pcloud.com'], path: '/dl/abc' }));
      }
      return Promise.resolve(new Response('gone', { status: 404, statusText: 'Not Found' }));
    });

    await expect(makeClient(fetchImpl as unknown as typeof fetch).downloadBlob(42)).rejects.toMatchObject({
      name: 'PCloudError',
      status: 404
    });
  });
});
