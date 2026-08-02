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

    it('allows reporting reading activity', async () => {
      await fetchJson(`/api/shelves/${SHELF}/reading_activity?book_id=abc`, { method: 'POST' });
      expect(fetchMock).toHaveBeenCalledOnce();
    });

    it('rejects a non-POST to the allowlisted path', async () => {
      await expect(
        fetchJson(`/api/shelves/${SHELF}/reading_activity`, { method: 'DELETE' })
      ).rejects.toThrow(ApiError);
      expect(fetchMock).not.toHaveBeenCalled();
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
      ['server setting', '/api/setting/cover_to_jpg', 'POST']
    ])('rejects %s', async (_label, path, method) => {
      await expect(fetchJson(path, { method })).rejects.toThrow(ApiError);
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it('does not let an allowlisted suffix elsewhere in the path through', async () => {
      await expect(
        fetchJson(`/api/shelves/${SHELF}/books/reading_activity`, { method: 'POST' })
      ).rejects.toThrow(ApiError);
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
        fetchJson(`/api/shelves/${SHELF}/reading_activity`, { method: 'POST' })
      ).rejects.toThrow(ApiError);
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });
});
