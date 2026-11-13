import { describe, expect, it, vi, beforeEach } from 'vitest';

const { fetchJsonMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn()
}));

vi.mock('./client', async () => {
  const actual = await vi.importActual<typeof import('./client')>('./client');
  return {
    ApiError: actual.ApiError,
    buildShelfApiPath: (path: string) => path,
    buildApiUrl: (path: string) => path,
    fetchBlob: vi.fn(),
    fetchJson: fetchJsonMock,
    fetchText: vi.fn(),
    isMockApiMode: () => false
  };
});

const { ApiError } = await import('./client');
const {
  updateBook,
  transferBook,
  startFingerprintSources,
  getSimilarBookPairs,
  FingerprintSweepBusyError
} = await import('./books');

describe('getSimilarBookPairs', () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
  });

  it('surfaces every over-budget estimate field to the caller', async () => {
    fetchJsonMock.mockResolvedValueOnce({
      status: 'too_large',
      total: 12,
      fingerprinted: 10,
      pairs: 45,
      work: 2_000_000_000,
      seconds: 60,
      budget: 1_073_741_824
    });

    const result = getSimilarBookPairs(0.15);
    await expect(result).rejects.toMatchObject({
      name: 'SimilarTooLargeError',
      total: 12,
      fingerprinted: 10,
      pairs: 45,
      work: 2_000_000_000,
      seconds: 60,
      budget: 1_073_741_824
    });
    expect(fetchJsonMock).toHaveBeenCalledWith('/books/similar?floor=0.15', undefined, undefined);
  });

  it('adds confirm=1 and a five-minute timeout only to confirmed comparisons', async () => {
    fetchJsonMock.mockResolvedValueOnce([]);

    await expect(getSimilarBookPairs(0.15, true)).resolves.toEqual([]);

    expect(fetchJsonMock).toHaveBeenCalledWith(
      '/books/similar?floor=0.15&confirm=1',
      undefined,
      { timeoutMs: 300_000 }
    );
  });
});

describe('transferBook', () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
  });

  it('posts the normalized destination and returns the scheduled chain id', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-1' });

    const id = await transferBook('book-1', 'shelf-b', 'fiction/scifi', 'move');

    expect(id).toBe('chain-1');
    const [path, init] = fetchJsonMock.mock.calls[0];
    expect(path).toBe('/books/book-1/transfers');
    expect(init).toMatchObject({ method: 'POST' });
    expect(JSON.parse((init as { body: string }).body)).toEqual({
      mode: 'move',
      target_shelf: 'shelf-b',
      target_folder: ['fiction', 'scifi']
    });
  });

  it('sends an empty folder array for a root destination', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-2' });

    await transferBook('book-1', 'shelf-b', '', 'copy');

    const [, init] = fetchJsonMock.mock.calls[0];
    expect(JSON.parse((init as { body: string }).body).target_folder).toEqual([]);
  });

  it('attaches to the chain a 409 already-running body names', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(JSON.stringify({ taskchain_id: 'chain-running' }), { status: 409 })
    );

    await expect(transferBook('book-1', 'shelf-b', '', 'move')).resolves.toBe('chain-running');
  });

  it('propagates a plain-text 409 conflict', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError('the target shelf already holds a book with this ID; the move would overwrite it', {
        status: 409
      })
    );

    await expect(transferBook('book-1', 'shelf-b', '', 'move')).rejects.toThrow(
      'the target shelf already holds a book with this ID'
    );
  });
});

describe('startFingerprintSources', () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
  });

  it('posts the incremental sweep and adopts a 409 already-running chain', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-1' });

    const id = await startFingerprintSources();

    expect(id).toBe('chain-1');
    const [path, init, options] = fetchJsonMock.mock.calls[0];
    expect(path).toBe('/source-fingerprints');
    expect(init).toMatchObject({ method: 'POST' });
    expect(options).toEqual({ acceptStatuses: [409] });
  });

  it('posts a forced sweep with the force flag and does not accept a 409', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-2' });

    const id = await startFingerprintSources(true);

    expect(id).toBe('chain-2');
    const [path, init, options] = fetchJsonMock.mock.calls[0];
    expect(path).toBe('/source-fingerprints?force=true');
    expect(init).toMatchObject({ method: 'POST' });
    expect(options).toBeUndefined();
  });

  it('maps a forced 409 to a FingerprintSweepBusyError', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(JSON.stringify({ taskchain_id: 'chain-incremental' }), { status: 409 })
    );

    await expect(startFingerprintSources(true)).rejects.toBeInstanceOf(FingerprintSweepBusyError);
  });

  it('propagates a non-conflict failure from a forced sweep unchanged', async () => {
    fetchJsonMock.mockRejectedValueOnce(new ApiError('boom', { status: 500 }));

    await expect(startFingerprintSources(true)).rejects.toThrow('boom');
  });
});

describe('updateBook series', () => {
  beforeEach(() => {
    fetchJsonMock.mockReset();
  });

  function backendBook(series?: { name: string; index: number }) {
    return {
      meta: { id: 'book-1', title: 'Saga', authors: [], language: '', format: 'txt', cover: '', series },
      folder: []
    };
  }

  it('sends series null to clear it and reads the series the server returns', async () => {
    fetchJsonMock.mockResolvedValueOnce(backendBook());
    const cleared = await updateBook('book-1', { series: null });
    expect(JSON.parse((fetchJsonMock.mock.calls[0][1] as { body: string }).body)).toEqual({ series: null });
    expect(cleared.series).toBeUndefined();

    fetchJsonMock.mockResolvedValueOnce(backendBook({ name: 'Saga', index: 1.5 }));
    const set = await updateBook('book-1', { series: { name: 'Saga', index: 1.5 } });
    expect(set.series).toEqual({ name: 'Saga', index: 1.5 });
  });
});
