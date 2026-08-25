import { describe, expect, it, vi } from 'vitest';

const { fetchJsonMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn()
}));

vi.mock('./client', async () => {
  const actual = await vi.importActual<typeof import('./client')>('./client');
  return {
    ApiError: actual.ApiError,
    fetchJson: fetchJsonMock,
    buildShelfApiPath: (path: string) => path,
    isMockApiMode: () => false
  };
});

const { ApiError } = await import('./client');
const { createFolder, transferFolder } = await import('./folders');

describe('transferFolder', () => {
  it('posts the source and target folders as segment arrays and returns the chain id', async () => {
    fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-1' });

    const id = await transferFolder('Fiction/SciFi', 'shelf-b', 'Archive/SciFi', 'move');

    expect(id).toBe('chain-1');
    const [path, init] = fetchJsonMock.mock.calls[0];
    expect(path).toBe('/folder-transfers');
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      mode: 'move',
      source_folder: ['Fiction', 'SciFi'],
      target_shelf: 'shelf-b',
      target_folder: ['Archive', 'SciFi']
    });
  });

  it('attaches to the already-running chain when the server answers 409 with its id', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(JSON.stringify({ taskchain_id: 'chain-running' }), { status: 409 })
    );

    await expect(transferFolder('Fiction', 'shelf-b', 'Fiction', 'copy')).resolves.toBe('chain-running');
  });

  it('raises a typed conflict when the target already holds a folder with this name', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(
        JSON.stringify({ error: 'target_folder_conflict', message: 'already holds a folder' }),
        { status: 409 }
      )
    );

    await expect(transferFolder('Fiction', 'shelf-b', 'Fiction', 'copy')).rejects.toMatchObject({
      name: 'FolderTransferConflictError',
      kind: 'target_folder_conflict'
    });
  });

  it('carries the colliding book ids on a move book-id conflict', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(
        JSON.stringify({
          error: 'book_id_conflict',
          message: 'would overwrite them',
          conflicting_book_ids: ['book-1', 'book-2']
        }),
        { status: 409 }
      )
    );

    await expect(transferFolder('Fiction', 'shelf-b', 'Fiction', 'move')).rejects.toMatchObject({
      name: 'FolderTransferConflictError',
      kind: 'book_id_conflict',
      conflictingBookIDs: ['book-1', 'book-2']
    });
  });

  it('propagates a plain-text 409 (a read-only target) unchanged', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError('shelf is opened read-only; this PlainShelf instance cannot modify it', { status: 409 })
    );

    await expect(transferFolder('Fiction', 'shelf-b', 'Fiction', 'move')).rejects.toThrow(
      'shelf is opened read-only'
    );
  });
});

describe('createFolder', () => {
  // The server refuses a name the shelf scanner would skip with an explanation
  // the user needs; replacing every 400 with one fixed sentence hid it.
  it('surfaces the reason the server gave for a rejected name', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError('invalid folder name: hidden and system directory names are skipped', { status: 400 })
    );

    await expect(createFolder('@eaDir')).rejects.toThrow(
      'invalid folder name: hidden and system directory names are skipped'
    );
  });

  it('still reports a generic failure for a server error', async () => {
    fetchJsonMock.mockRejectedValueOnce(new ApiError('boom', { status: 500 }));

    await expect(createFolder('Fiction')).rejects.toThrow('Failed to create folder');
  });
});
