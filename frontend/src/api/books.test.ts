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
const { transferBook, startFingerprintSources, FingerprintSweepBusyError } = await import('./books');

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
      target_layer: ['fiction', 'scifi']
    });
  });

  it('sends an empty layer array for a root destination', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-2' });

    await transferBook('book-1', 'shelf-b', '', 'copy');

    const [, init] = fetchJsonMock.mock.calls[0];
    expect(JSON.parse((init as { body: string }).body).target_layer).toEqual([]);
  });

  // A transfer already in flight answers 409 with the running chain's id as JSON,
  // which is adopted so the caller attaches to it rather than failing.
  it('attaches to the chain a 409 already-running body names', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(JSON.stringify({ taskchain_id: 'chain-running' }), { status: 409 })
    );

    await expect(transferBook('book-1', 'shelf-b', '', 'move')).resolves.toBe('chain-running');
  });

  // Every other 409 (target holds this id, target/source read-only) is a
  // plain-text conflict that must surface with the server's message.
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
    // The incremental request accepts a 409 so a second press attaches to the
    // sweep already running.
    expect(options).toEqual({ acceptStatuses: [409] });
  });

  it('posts a forced sweep with the force flag and does not accept a 409', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-2' });

    const id = await startFingerprintSources(true);

    expect(id).toBe('chain-2');
    const [path, init, options] = fetchJsonMock.mock.calls[0];
    expect(path).toBe('/source-fingerprints?force=true');
    expect(init).toMatchObject({ method: 'POST' });
    // No acceptStatuses: a 409 must reject so the force path can classify it
    // rather than silently adopt whatever chain is already running.
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
