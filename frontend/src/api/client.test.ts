import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { isMobileRuntimeMock } = vi.hoisted(() => ({
  isMobileRuntimeMock: vi.fn()
}));

vi.mock('@/providers/runtime', () => ({
  isMobileRuntime: isMobileRuntimeMock
}));

// The suite runs in the default node environment, but client.ts reads
// window.plainshelf / window.__PLAINSHELF_SECURITY__ while building headers.
// A minimal stub is enough; no DOM behavior is exercised here.
const storage = new Map<string, string>();
Object.defineProperty(globalThis, 'window', {
  configurable: true,
  writable: true,
  value: {
    localStorage: {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => void storage.set(key, value),
      removeItem: (key: string) => void storage.delete(key)
    }
  }
});

const { ApiError, fetchJson, setActiveShelfID } = await import('./client');

const SHELF = 'main';

function okResponse(): Response {
  return new Response(null, { status: 204 });
}

describe('assertWritableRequest', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    isMobileRuntimeMock.mockReset();
    isMobileRuntimeMock.mockReturnValue(false);
    (window as unknown as { __PLAINSHELF_READ_ONLY__?: boolean }).__PLAINSHELF_READ_ONLY__ = false;
    setActiveShelfID(SHELF);

    fetchMock = vi.fn().mockResolvedValue(okResponse());
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('in the mobile shell', () => {
    beforeEach(() => {
      isMobileRuntimeMock.mockReturnValue(true);
    });

    it('allows reads', async () => {
      await fetchJson(`/api/shelves/${SHELF}/books`);
      expect(fetchMock).toHaveBeenCalledOnce();
    });

    it.each([
      ['metadata edit', `/api/shelves/${SHELF}/books/abc`, 'PATCH'],
      ['cover upload', `/api/shelves/${SHELF}/books/abc/cover`, 'PUT'],
      ['cover removal', `/api/shelves/${SHELF}/books/abc/cover`, 'DELETE'],
      ['move to trash', `/api/shelves/${SHELF}/books/abc/trash`, 'POST'],
      ['import', `/api/shelves/${SHELF}/books/import`, 'POST'],
      ['create layer', `/api/shelves/${SHELF}/layers/fiction`, 'POST'],
      ['delete layer', `/api/shelves/${SHELF}/layers/fiction`, 'DELETE'],
      ['empty trash', `/api/shelves/${SHELF}/trash/empty`, 'POST'],
      ['batch operation', `/api/shelves/${SHELF}/book-batches`, 'POST'],
      ['server setting', '/api/setting/cover_to_jpg', 'POST'],
      // Reading time used to be the one write the mobile shell was allowed to
      // make; it is recorded on the device now, so no write is exempt.
      ['reading activity', `/api/shelves/${SHELF}/reading_activity`, 'POST']
    ])('rejects %s', async (_label, path, method) => {
      await expect(fetchJson(path, { method })).rejects.toThrow(ApiError);
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe('outside the mobile shell', () => {
    it('allows writes when the server is writable', async () => {
      await fetchJson(`/api/shelves/${SHELF}/books/abc`, { method: 'PATCH' });
      expect(fetchMock).toHaveBeenCalledOnce();
    });

    it('still rejects every write when the server is read-only', async () => {
      (window as unknown as { __PLAINSHELF_READ_ONLY__?: boolean }).__PLAINSHELF_READ_ONLY__ = true;

      await expect(
        fetchJson(`/api/shelves/${SHELF}/books/abc`, { method: 'PATCH' })
      ).rejects.toThrow(ApiError);
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  // Both gates exist to stop writes, and a rescan is not one: it walks the
  // shelf and changes nothing on it. The server draws the same exception for
  // the same route (isReadOnlySafeRequest in server/app.go).
  describe('a POST that writes nothing', () => {
    it('is allowed against a read-only server', async () => {
      (window as unknown as { __PLAINSHELF_READ_ONLY__?: boolean }).__PLAINSHELF_READ_ONLY__ = true;

      await fetchJson(`/api/shelves/${SHELF}/scans`, { method: 'POST' }, { readOnlySafe: true });
      expect(fetchMock).toHaveBeenCalledOnce();
    });

    it('is allowed from the mobile shell', async () => {
      isMobileRuntimeMock.mockReturnValue(true);

      await fetchJson(`/api/shelves/${SHELF}/scans`, { method: 'POST' }, { readOnlySafe: true });
      expect(fetchMock).toHaveBeenCalledOnce();
    });

    // The exemption is opt-in per request, so it cannot leak to the write next
    // to it on the same route prefix.
    it('does not exempt a request that omits the flag', async () => {
      (window as unknown as { __PLAINSHELF_READ_ONLY__?: boolean }).__PLAINSHELF_READ_ONLY__ = true;

      await expect(
        fetchJson(`/api/shelves/${SHELF}/scans`, { method: 'POST' })
      ).rejects.toThrow(ApiError);
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });
});
