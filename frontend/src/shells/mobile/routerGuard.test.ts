import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { NavigationGuardWithThis, Router } from 'vue-router';
import type { DownloadState } from '@/types/book';

const { loadShelfEntriesMock, isShelfEntryUsableMock, getDownloadStateMock, providerMock } =
  vi.hoisted(() => ({
    loadShelfEntriesMock: vi.fn(),
    isShelfEntryUsableMock: vi.fn(),
    getDownloadStateMock: vi.fn(),
    providerMock: {} as { getDownloadState?: (id: string) => Promise<DownloadState> }
  }));

vi.mock('@/providers/mobileConfig', () => ({
  loadShelfEntries: loadShelfEntriesMock,
  isShelfEntryUsable: isShelfEntryUsableMock
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => providerMock
}));

const { installMobileRouterGuards } = await import('./routerGuard');

// Captures the guard `installMobileRouterGuards` registers so it can be invoked
// with a synthetic destination, without standing up a real router.
function captureGuard(): NavigationGuardWithThis<undefined> {
  let guard: NavigationGuardWithThis<undefined> | null = null;
  const router = {
    beforeEach: (fn: NavigationGuardWithThis<undefined>) => {
      guard = fn;
    }
  } as unknown as Router;
  installMobileRouterGuards(router);
  if (!guard) {
    throw new Error('guard was not registered');
  }
  return guard;
}

function reader(id: string, query: Record<string, string> = {}) {
  return { name: 'reader', params: { id }, query, path: `/reader/${id}`, hash: '' };
}

beforeEach(() => {
  loadShelfEntriesMock.mockReset().mockResolvedValue({
    entries: [{ id: 'e' }],
    activeEntryID: 'e'
  });
  isShelfEntryUsableMock.mockReset().mockResolvedValue(true);
  getDownloadStateMock.mockReset();
  providerMock.getDownloadState = getDownloadStateMock;
});

describe('mobile reader download gate', () => {
  it('redirects a not-downloaded book to its detail page with the prompt flag', async () => {
    getDownloadStateMock.mockResolvedValue('not_downloaded' satisfies DownloadState);
    const guard = captureGuard();

    const result = await guard.call(undefined, reader('book-1', { mobile: '1' }) as never, {} as never);

    expect(result).toEqual({
      name: 'book-detail',
      params: { id: 'book-1' },
      query: { mobile: '1', downloadRequired: '1' }
    });
    expect(getDownloadStateMock).toHaveBeenCalledWith('book-1');
  });

  it('lets a downloaded book open the reader', async () => {
    getDownloadStateMock.mockResolvedValue('downloaded' satisfies DownloadState);
    const guard = captureGuard();

    const result = await guard.call(undefined, reader('book-1') as never, {} as never);

    expect(result).toBe(true);
  });

  it('treats a state-read failure as not downloaded so the requirement holds', async () => {
    getDownloadStateMock.mockRejectedValue(new Error('cache miss'));
    const guard = captureGuard();

    const result = await guard.call(undefined, reader('book-1') as never, {} as never);

    expect(result).toMatchObject({ name: 'book-detail', params: { id: 'book-1' } });
  });

  it('does not gate when the provider has no download concept', async () => {
    providerMock.getDownloadState = undefined;
    const guard = captureGuard();

    const result = await guard.call(undefined, reader('book-1') as never, {} as never);

    expect(result).toBe(true);
  });
});
