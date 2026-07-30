import { effectScope, nextTick, ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import bookcover from '@/assets/bookcover.svg';

const { getBookCoverMock, getBookCoverUrlMock, isMobileRuntimeMock } = vi.hoisted(() => ({
  getBookCoverMock: vi.fn(),
  getBookCoverUrlMock: vi.fn(),
  isMobileRuntimeMock: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    getBookCover: getBookCoverMock,
    getBookCoverUrl: getBookCoverUrlMock
  })
}));

vi.mock('@/providers/runtime', () => ({
  isMobileRuntime: isMobileRuntimeMock
}));

const { useCoverSrc } = await import('./useCoverSrc');

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('useCoverSrc', () => {
  beforeEach(() => {
    getBookCoverMock.mockReset();
    getBookCoverUrlMock.mockReset();
    isMobileRuntimeMock.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('mobile: resolves src to a blob: object URL fetched through the provider', async () => {
    isMobileRuntimeMock.mockReturnValue(true);
    getBookCoverMock.mockResolvedValue(new Blob(['fake-cover-bytes'], { type: 'image/png' }));

    const scope = effectScope();
    let result!: ReturnType<typeof useCoverSrc>;
    scope.run(() => {
      result = useCoverSrc(
        () => 'book-mobile-1',
        () => 'http://192.168.1.5:20000/api/shelves/s1/books/book-mobile-1/cover'
      );
    });

    await flush();
    await nextTick();

    expect(result.src.value.startsWith('blob:')).toBe(true);
    expect(getBookCoverMock).toHaveBeenCalledWith('book-mobile-1');
    expect(getBookCoverUrlMock).not.toHaveBeenCalled();

    scope.stop();
  });

  it('desktop: src passes through coverUrl unchanged, no blob is created', async () => {
    isMobileRuntimeMock.mockReturnValue(false);

    const coverUrl = 'https://example.com/book-desktop-1/cover.jpg';
    const scope = effectScope();
    let result!: ReturnType<typeof useCoverSrc>;
    scope.run(() => {
      result = useCoverSrc(() => 'book-desktop-1', () => coverUrl);
    });

    await flush();
    await nextTick();

    expect(result.src.value).toBe(coverUrl);
    expect(getBookCoverMock).not.toHaveBeenCalled();

    scope.stop();
  });

  it('cacheKey change triggers a refetch (mobile) and yields a fresh object URL', async () => {
    isMobileRuntimeMock.mockReturnValue(true);
    getBookCoverMock
      .mockResolvedValueOnce(new Blob(['v1'], { type: 'image/png' }))
      .mockResolvedValueOnce(new Blob(['v2'], { type: 'image/png' }));

    const cacheKey = ref<number | undefined>(1);
    const scope = effectScope();
    let result!: ReturnType<typeof useCoverSrc>;
    scope.run(() => {
      result = useCoverSrc(
        () => 'book-cachekey-1',
        () => 'http://server/api/shelves/s1/books/book-cachekey-1/cover',
        () => cacheKey.value
      );
    });

    await flush();
    await nextTick();
    const firstSrc = result.src.value;
    expect(firstSrc.startsWith('blob:')).toBe(true);

    cacheKey.value = 2;
    await nextTick();
    await flush();
    await nextTick();

    expect(result.src.value.startsWith('blob:')).toBe(true);
    expect(result.src.value).not.toBe(firstSrc);
    expect(getBookCoverMock).toHaveBeenCalledTimes(2);

    scope.stop();
  });

  it('dispose releases the object URL only once every consumer with the same key has disposed', async () => {
    isMobileRuntimeMock.mockReturnValue(true);
    getBookCoverMock.mockResolvedValue(new Blob(['shared'], { type: 'image/png' }));
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL');

    const bookId = () => 'book-refcount-1';
    const coverUrl = () => 'http://server/api/shelves/s1/books/book-refcount-1/cover';

    const scopeA = effectScope();
    let resultA!: ReturnType<typeof useCoverSrc>;
    scopeA.run(() => {
      resultA = useCoverSrc(bookId, coverUrl);
    });

    const scopeB = effectScope();
    let resultB!: ReturnType<typeof useCoverSrc>;
    scopeB.run(() => {
      resultB = useCoverSrc(bookId, coverUrl);
    });

    await flush();
    await nextTick();

    expect(resultA.src.value.startsWith('blob:')).toBe(true);
    expect(resultB.src.value).toBe(resultA.src.value);
    // Same key: the second consumer must reuse the in-flight/resolved fetch,
    // not trigger a second network call.
    expect(getBookCoverMock).toHaveBeenCalledTimes(1);

    scopeA.stop();
    expect(revokeSpy).not.toHaveBeenCalled();

    scopeB.stop();
    expect(revokeSpy).toHaveBeenCalledTimes(1);
  });

  it('falls back to bookcover when the cover fetch fails (mobile)', async () => {
    isMobileRuntimeMock.mockReturnValue(true);
    getBookCoverMock.mockRejectedValue(new Error('network error'));

    const scope = effectScope();
    let result!: ReturnType<typeof useCoverSrc>;
    scope.run(() => {
      result = useCoverSrc(
        () => 'book-fail-1',
        () => 'http://server/api/shelves/s1/books/book-fail-1/cover'
      );
    });

    await flush();
    await nextTick();

    expect(result.src.value).toBe(bookcover);

    scope.stop();
  });

  it('handleError resets src back to bookcover', async () => {
    isMobileRuntimeMock.mockReturnValue(true);
    getBookCoverMock.mockResolvedValue(new Blob(['ok'], { type: 'image/png' }));

    const scope = effectScope();
    let result!: ReturnType<typeof useCoverSrc>;
    scope.run(() => {
      result = useCoverSrc(
        () => 'book-error-handler-1',
        () => 'http://server/api/shelves/s1/books/book-error-handler-1/cover'
      );
    });

    await flush();
    await nextTick();
    expect(result.src.value.startsWith('blob:')).toBe(true);

    result.handleError();
    expect(result.src.value).toBe(bookcover);

    scope.stop();
  });
});
