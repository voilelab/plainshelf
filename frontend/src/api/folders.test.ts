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
const { createFolder, moveFolder, renameFolder, transferFolder } = await import('./folders');

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

  // A folder conflict collides on no book, so the server sends an empty list
  // rather than omitting the field.
  it('raises a typed conflict when the target already holds a folder with this name', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(
        JSON.stringify({
          error: 'target_folder_conflict',
          message: 'already holds a folder',
          conflicting_book_ids: []
        }),
        { status: 409 }
      )
    );

    await expect(transferFolder('Fiction', 'shelf-b', 'Fiction', 'copy')).rejects.toMatchObject({
      name: 'FolderTransferConflictError',
      kind: 'target_folder_conflict',
      conflictingBookIDs: []
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

/*
Moving or renaming a folder out of a subtree shelf.json marks as adult content
unmarks everything below it, so the server refuses the change with its own 409
body until the caller confirms. All three routes carry the flag as ?confirm=1
because the rename's body has room for a folder name and nothing else.
*/
describe('folder changes that would unhide marked content', () => {
  const refusal = () =>
    new ApiError(
      JSON.stringify({
        error: 'nsfw_reveal_requires_confirmation',
        message: 'this folder holds content marked as adult',
        hidden_books: 3
      }),
      { status: 409 }
    );

  const changes = {
    moveFolder: (confirm?: boolean) => moveFolder('Fiction', 'Archive', { confirm }),
    renameFolder: (confirm?: boolean) => renameFolder('Fiction', 'Library', { confirm }),
    transferFolder: (confirm?: boolean) =>
      transferFolder('Fiction', 'shelf-b', 'Imported', 'copy', { confirm })
  };

  for (const [name, change] of Object.entries(changes)) {
    it(`${name} raises a typed confirmation request carrying the book count`, async () => {
      fetchJsonMock.mockRejectedValueOnce(refusal());

      await expect(change()).rejects.toMatchObject({
        name: 'NsfwRevealConfirmationError',
        hiddenBooks: 3
      });
    });

    it(`${name} asks again with confirm=1 once the user has agreed`, async () => {
      fetchJsonMock.mockClear();
      fetchJsonMock.mockResolvedValueOnce({ taskchain_id: 'chain-1' });

      await change(true);

      expect(fetchJsonMock.mock.calls[0][0]).toMatch(/\?confirm=1$/);
    });
  }

  // hidden_books is 0 when the whole disclosure is a folder name, and a client
  // that read that as "nothing would change" would skip the question entirely.
  it('keeps a count of zero rather than falling back to the generic 409 path', async () => {
    fetchJsonMock.mockRejectedValueOnce(
      new ApiError(
        JSON.stringify({
          error: 'nsfw_reveal_requires_confirmation',
          message: 'the folder itself would become visible',
          hidden_books: 0
        }),
        { status: 409 }
      )
    );

    await expect(moveFolder('Fiction', 'Archive')).rejects.toMatchObject({
      name: 'NsfwRevealConfirmationError',
      hiddenBooks: 0
    });
  });
});
