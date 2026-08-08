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

    await client.listFolder(7);

    expect(fetchImpl.mock.calls[0][0]).toContain('https://eapi.pcloud.com/listfolder');
  });
});

describe('PCloudClient.call', () => {
  it('sends the token as a Bearer header rather than in the URL', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0, metadata: { name: 'x', isfolder: true } }));
    await makeClient(fetchImpl as unknown as typeof fetch).listFolder(7);

    const [url, init] = fetchImpl.mock.calls[0];
    expect(String(url)).not.toContain(TOKEN);
    expect((init as RequestInit).headers).toMatchObject({ Authorization: `Bearer ${TOKEN}` });
  });

  it('turns a non-zero result into a PCloudError carrying the code', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 2005, error: 'Directory does not exist.' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(99)).rejects.toMatchObject(
      { name: 'PCloudError', result: 2005 }
    );
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('retries a 4xxx rate-limit result and then succeeds', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ result: 4000, error: 'Too many requests.' }))
      .mockResolvedValueOnce(jsonResponse({ result: 0, metadata: { name: 'shelf', isfolder: true } }));

    const meta = await makeClient(fetchImpl as unknown as typeof fetch).listFolder(7);

    expect(meta.name).toBe('shelf');
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it('does not retry a permanent error', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 1101, error: 'Not supported.' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(7)).rejects.toBeInstanceOf(
      PCloudError
    );
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('retries a dropped connection', async () => {
    const fetchImpl = vi
      .fn()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(jsonResponse({ result: 0, metadata: { name: 'shelf', isfolder: true } }));

    const meta = await makeClient(fetchImpl as unknown as typeof fetch).listFolder(7);

    expect(meta.name).toBe('shelf');
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it('gives up on a persistent network failure once the attempt budget is spent', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(7)).rejects.toBeInstanceOf(
      PCloudError
    );
    expect(fetchImpl).toHaveBeenCalledTimes(3);
  });

  it('does not retry when the caller cancels', async () => {
    const controller = new AbortController();
    const fetchImpl = vi.fn().mockImplementation(() => {
      controller.abort();
      return Promise.reject(new DOMException('Aborted', 'AbortError'));
    });

    await expect(
      makeClient(fetchImpl as unknown as typeof fetch).listFolder(7, { signal: controller.signal })
    ).rejects.toBeTruthy();
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  // The regression this pins: a reply with no `result` used to be read as
  // success, so a non-pCloud reply flowed on as an empty listing and only
  // surfaced much later as a confusing "folder not found".
  it('refuses a reply that carries no pCloud result', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ hello: 'not pcloud' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(7)).rejects.toThrow(
      /did not come from the pCloud API/
    );
  });

  it('quotes the unexpected reply back, truncated', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ padding: 'x'.repeat(500) }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(7)).rejects.toThrow(/…/);
  });

  it('refuses a success that carries no folder', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0 }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(7)).rejects.toThrow(
      /returned no folder/
    );
  });

  it('reports an HTTP failure with its status', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response('nope', { status: 401, statusText: 'Unauthorized' }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).listFolder(7)).rejects.toMatchObject({
      name: 'PCloudError',
      status: 401
    });
  });
});

describe('PCloudClient.getUserInfo', () => {
  it('returns the account identity and addresses the userinfo method', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse({ result: 0, userid: 42, email: 'reader@example.com', emailverified: true })
    );

    const info = await makeClient(fetchImpl as unknown as typeof fetch).getUserInfo();

    expect(info).toMatchObject({ userid: 42, email: 'reader@example.com' });
    expect(String(fetchImpl.mock.calls[0][0])).toContain('/userinfo');
  });

  it('rejects a success response with no account identity', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0 }));

    await expect(makeClient(fetchImpl as unknown as typeof fetch).getUserInfo()).rejects.toThrow(
      /no account identity/
    );
  });
});

describe('PCloudClient.listFolderRecursive', () => {
  it('asks for the whole tree in one request', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ result: 0, metadata: { name: 'shelf', isfolder: true } }));
    await makeClient(fetchImpl as unknown as typeof fetch).listFolderRecursive({ folderid: 7 });

    expect(fetchImpl).toHaveBeenCalledTimes(1);
    expect(String(fetchImpl.mock.calls[0][0])).toContain('recursive=1');
  });

  it('falls back to a manual account-root walk when recursive listing is refused', async () => {
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
    // Prefer the documented recursive request, then fall back for API variants
    // that reject it.
    const recursiveRootCalls = fetchImpl.mock.calls.filter((call) => {
      const url = new URL(String(call[0]));
      return url.searchParams.get('folderid') === '0' && url.searchParams.get('recursive') === '1';
    });
    expect(recursiveRootCalls).toHaveLength(1);
  });

  it('falls back to a manual walk if a non-root folder refuses recursion', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = new URL(String(input));
      if (url.searchParams.get('recursive') === '1') {
        return Promise.resolve(jsonResponse({ result: 1101, error: 'Not supported.' }));
      }
      return Promise.resolve(
        jsonResponse({
          result: 0,
          metadata: { name: 'shelf', isfolder: true, folderid: 7, contents: [] }
        })
      );
    });

    const tree = await makeClient(fetchImpl as unknown as typeof fetch).listFolderRecursive({ folderid: 7 });

    expect(tree.name).toBe('shelf');
  });

  // The regression this pins: pCloud answers "Directory does not exist" for a
  // listfolder addressed by path, even for a folder that plainly exists, so
  // every folder must be addressed by id.
  it('never addresses listfolder by path', async () => {
    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = new URL(String(input));
      const folderid = url.searchParams.get('folderid');
      if (folderid === '0') {
        return Promise.resolve(
          jsonResponse({
            result: 0,
            metadata: { name: '/', isfolder: true, folderid: 0, contents: [{ name: 'test1', isfolder: true, folderid: 7 }] }
          })
        );
      }
      return Promise.resolve(
        jsonResponse({ result: 0, metadata: { name: 'test1', isfolder: true, folderid: 7, contents: [] } })
      );
    });

    await makeClient(fetchImpl as unknown as typeof fetch).listFolderRecursive({ path: '/test1' });

    for (const call of fetchImpl.mock.calls) {
      expect(new URL(String(call[0])).searchParams.has('path')).toBe(false);
    }
  });
});

describe('PCloudClient.resolveFolderID', () => {
  function treeFetch(tree: Record<string, Array<{ name: string; folderid?: number; isfolder?: boolean }>>) {
    return vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const folderid = new URL(String(input)).searchParams.get('folderid') ?? '0';
      const contents = tree[folderid];
      if (!contents) {
        return Promise.resolve(jsonResponse({ result: 2005, error: 'Directory does not exist.' }));
      }
      return Promise.resolve(
        jsonResponse({
          result: 0,
          metadata: {
            name: folderid,
            isfolder: true,
            folderid: Number(folderid),
            contents: contents.map((item) => ({ isfolder: true, ...item }))
          }
        })
      );
    });
  }

  it('walks segment by segment from the account root', async () => {
    const fetchImpl = treeFetch({
      '0': [{ name: 'test1', folderid: 7 }],
      '7': [{ name: 'books', folderid: 8 }],
      '8': []
    });

    await expect(makeClient(fetchImpl as unknown as typeof fetch).resolveFolderID('/test1/books')).resolves.toBe(8);
  });

  it('ignores leading, trailing and repeated slashes', async () => {
    const fetchImpl = treeFetch({ '0': [{ name: 'test1', folderid: 7 }], '7': [] });
    const client = makeClient(fetchImpl as unknown as typeof fetch);

    await expect(client.resolveFolderID('/test1')).resolves.toBe(7);
    await expect(client.resolveFolderID('test1/')).resolves.toBe(7);
    await expect(client.resolveFolderID('//test1//')).resolves.toBe(7);
  });

  it('resolves an empty path to the account root', async () => {
    const fetchImpl = treeFetch({ '0': [] });

    await expect(makeClient(fetchImpl as unknown as typeof fetch).resolveFolderID('/')).resolves.toBe(0);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('names the segment that is missing, and where', async () => {
    const fetchImpl = treeFetch({ '0': [{ name: 'test1', folderid: 7 }], '7': [] });
    const client = makeClient(fetchImpl as unknown as typeof fetch);

    await expect(client.resolveFolderID('/nope')).rejects.toThrow(/"nope".*account root/);
    await expect(client.resolveFolderID('/test1/books')).rejects.toThrow(/"books".*"\/test1"/);
  });

  // Without this the message cannot tell a typo from "this device is signed
  // into a different pCloud account", which look identical to the user.
  it('lists the folders that level does contain', async () => {
    const fetchImpl = treeFetch({
      '0': [
        { name: 'Photos', folderid: 5 },
        { name: 'Documents', folderid: 6 }
      ]
    });

    await expect(
      makeClient(fetchImpl as unknown as typeof fetch).resolveFolderID('/test1')
    ).rejects.toThrow(/It contains "Documents", "Photos"\./);
  });

  it('says so plainly when the level has no folders', async () => {
    const fetchImpl = treeFetch({ '0': [] });

    await expect(
      makeClient(fetchImpl as unknown as typeof fetch).resolveFolderID('/test1')
    ).rejects.toThrow(/contains no folders at all/);
  });

  it('caps a long list of folder names', async () => {
    const many = Array.from({ length: 14 }, (_, index) => ({
      name: `folder-${String(index).padStart(2, '0')}`,
      folderid: 100 + index
    }));
    const fetchImpl = treeFetch({ '0': many });

    await expect(
      makeClient(fetchImpl as unknown as typeof fetch).resolveFolderID('/test1')
    ).rejects.toThrow(/and 4 more\./);
  });

  it('does not match a file of the same name', async () => {
    const fetchImpl = treeFetch({ '0': [{ name: 'test1', isfolder: false }] });

    await expect(
      makeClient(fetchImpl as unknown as typeof fetch).resolveFolderID('/test1')
    ).rejects.toBeInstanceOf(PCloudError);
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

  it('keeps the time budget and cancellation in force while the body is read', async () => {
    // fetch resolves on headers, but the body is the slow part. Tearing the
    // timer and abort listener down between the two would leave a stalled
    // transfer unbounded, so cleanup must not run until the body is consumed.
    const controller = new AbortController();
    const removeListener = vi.spyOn(controller.signal, 'removeEventListener');
    let removalsAtHeaders = -1;
    let removalsDuringBody = -1;

    const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('getfilelink')) {
        return Promise.resolve(jsonResponse({ result: 0, hosts: ['c1.pcloud.com'], path: '/dl/abc' }));
      }

      removalsAtHeaders = removeListener.mock.calls.length;
      return Promise.resolve({
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () => {
          removalsDuringBody = removeListener.mock.calls.length;
          return 'book text';
        }
      } as unknown as Response);
    });

    const text = await makeClient(fetchImpl as unknown as typeof fetch).downloadText(42, controller.signal);

    expect(text).toBe('book text');
    expect(removalsDuringBody).toBe(removalsAtHeaders);
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
