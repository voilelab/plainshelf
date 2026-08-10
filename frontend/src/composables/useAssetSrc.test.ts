import { effectScope, nextTick, ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { getSourceAssetMock, providerRef } = vi.hoisted(() => ({
  getSourceAssetMock: vi.fn(),
  providerRef: { current: {} as Record<string, unknown> }
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => providerRef.current
}));

const { useAssetSrc } = await import('./useAssetSrc');

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('useAssetSrc', () => {
  beforeEach(() => {
    getSourceAssetMock.mockReset();
    providerRef.current = { getSourceAsset: getSourceAssetMock };
    vi.spyOn(URL, 'revokeObjectURL');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('resolves src to a blob: object URL fetched through the provider', async () => {
    getSourceAssetMock.mockResolvedValue(new Blob(['fake-png-bytes'], { type: 'image/png' }));

    const scope = effectScope();
    let result!: ReturnType<typeof useAssetSrc>;
    scope.run(() => {
      result = useAssetSrc(
        () => 'book-1',
        () => 'src-1',
        () => 'img-0001.png'
      );
    });

    await flush();
    await nextTick();

    expect(result.src.value.startsWith('blob:')).toBe(true);
    expect(result.failed.value).toBe(false);
    expect(getSourceAssetMock).toHaveBeenCalledWith('book-1', 'src-1', 'img-0001.png');

    scope.stop();
  });

  // A backend that cannot reach assets at all must not leave the reader
  // waiting: the caller shows the alt text as soon as it knows.
  it('fails without a request when the provider cannot serve assets', async () => {
    providerRef.current = {};

    const scope = effectScope();
    let result!: ReturnType<typeof useAssetSrc>;
    scope.run(() => {
      result = useAssetSrc(
        () => 'book-1',
        () => 'src-1',
        () => 'img-0001.png'
      );
    });

    await flush();
    await nextTick();

    expect(result.failed.value).toBe(true);
    expect(result.src.value).toBe('');
    expect(getSourceAssetMock).not.toHaveBeenCalled();

    scope.stop();
  });

  it('fails when the fetch rejects', async () => {
    getSourceAssetMock.mockRejectedValue(new Error('offline'));

    const scope = effectScope();
    let result!: ReturnType<typeof useAssetSrc>;
    scope.run(() => {
      result = useAssetSrc(
        () => 'book-1',
        () => 'src-missing',
        () => 'img-0404.png'
      );
    });

    await flush();
    await nextTick();

    expect(result.failed.value).toBe(true);
    expect(result.src.value).toBe('');

    scope.stop();
  });

  // Two illustrations of the same file in one chapter must share one blob, and
  // the blob must survive until the last of them is gone.
  it('shares one object URL across consumers and revokes it at the last release', async () => {
    getSourceAssetMock.mockResolvedValue(new Blob(['shared'], { type: 'image/png' }));

    const scopeA = effectScope();
    const scopeB = effectScope();
    let a!: ReturnType<typeof useAssetSrc>;
    let b!: ReturnType<typeof useAssetSrc>;
    const args = [() => 'book-1', () => 'src-1', () => 'img-shared.png'] as const;
    scopeA.run(() => {
      a = useAssetSrc(...args);
    });
    scopeB.run(() => {
      b = useAssetSrc(...args);
    });

    await flush();
    await nextTick();

    expect(a.src.value).toBe(b.src.value);
    expect(getSourceAssetMock).toHaveBeenCalledTimes(1);

    scopeA.stop();
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();

    scopeB.stop();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(a.src.value);
  });

  it('re-resolves when the asset name changes', async () => {
    getSourceAssetMock.mockResolvedValue(new Blob(['bytes'], { type: 'image/png' }));

    const name = ref('img-0001.png');
    const scope = effectScope();
    let result!: ReturnType<typeof useAssetSrc>;
    scope.run(() => {
      result = useAssetSrc(
        () => 'book-1',
        () => 'src-1',
        () => name.value
      );
    });

    await flush();
    await nextTick();
    const first = result.src.value;

    name.value = 'img-0002.png';
    await nextTick();
    await flush();
    await nextTick();

    expect(result.src.value).not.toBe(first);
    expect(getSourceAssetMock).toHaveBeenLastCalledWith('book-1', 'src-1', 'img-0002.png');

    scope.stop();
  });
});
