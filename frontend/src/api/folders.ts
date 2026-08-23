import type { BookTransferMode } from './books';
import { ApiError, buildShelfApiPath, fetchJson, isMockApiMode } from './client';
import { FolderHttpError } from './folderErrors';
import { delay } from './mocks/latency';
import {
  addMockFolder,
  deleteMockFolder,
  getMockFolders,
  moveMockFolder,
  renameMockFolder
} from './mocks/folders';
import { mockTransferFolder } from './mocks/books';
import { normalizeFolderPath } from '@/utils/folders';

function normalizeFolderValue(value: unknown): string | null {
  if (typeof value === 'string') {
    const normalized = normalizeFolderPath(value);
    return normalized.length > 0 ? normalized : '/';
  }

  if (Array.isArray(value)) {
    const segments = value
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter((item) => item.length > 0);

    if (segments.length === 0) {
      return '/';
    }
    return normalizeFolderPath(segments.join('/'));
  }

  return null;
}

function foldersFromPath(path: string): string[] {
  const normalized = normalizeFolderPath(path);
  if (!normalized || normalized === '/') {
    return [];
  }
  return normalized.split('/').filter((segment) => segment.length > 0);
}

function encodeFolderPath(path: string): string {
  return path
    .split('/')
    .filter((segment) => segment.length > 0)
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

/**
 * Lists the folders of a shelf. Defaults to the active shelf; pass `shelfID` to
 * read another shelf's folders, which is what the cross-shelf transfer picker
 * needs — its destination folders belong to the target shelf, not the active one.
 */
export async function getFolders(shelfID?: string): Promise<string[]> {
  if (isMockApiMode()) {
    return delay(getMockFolders());
  }

  const data: unknown = await fetchJson<unknown>(buildShelfApiPath('/folders', shelfID), {
    method: 'GET'
  });
  if (!Array.isArray(data)) {
    throw new Error('Failed to fetch folders: invalid response format');
  }

  const unique = new Set<string>();
  for (const item of data) {
    const normalized = normalizeFolderValue(item);
    if (normalized) {
      unique.add(normalized);
    }
  }

  return Array.from(unique).sort((a, b) => a.localeCompare(b));
}

/** Which cross-shelf conflict a folder transfer was refused for. */
export type FolderTransferConflictKind = 'target_folder_conflict' | 'book_id_conflict';

/**
 * Thrown by {@link transferFolder} when the server refuses the transfer up front
 * with a 409 conflict body — either the target shelf already holds a folder with
 * this name, or (for a move) it already holds books with these IDs. A distinct
 * type, not a bare Error, so the modal can name which conflict it is and list the
 * colliding book IDs rather than showing one opaque message. The server's own
 * message is kept as `message` so the UI can fall back to it.
 */
export class FolderTransferConflictError extends Error {
  constructor(
    readonly kind: FolderTransferConflictKind,
    message: string,
    readonly conflictingBookIDs: string[] = []
  ) {
    super(message);
    this.name = 'FolderTransferConflictError';
  }
}

interface FolderTransferConflictBody {
  error?: string;
  message?: string;
  conflicting_book_ids?: string[];
  taskchain_id?: string;
}

/**
 * Copies or moves a whole folder — every book and sub-folder beneath it —
 * from the active shelf to `targetShelfID`, returning the id of the background
 * task chain to poll. The work is asynchronous because a folder can hold hundreds
 * of megabytes and either shelf may be a network mount, so the request must not
 * block on it.
 *
 * `sourceFolder` and `targetFolder` are '/'-joined paths. `targetFolder` is the full
 * destination path the folder lands at on the target shelf (it keeps its own name
 * when the caller nests it under a chosen parent); the server refuses a root
 * target. When the same transfer is already running the server answers 409 with
 * that chain's id, returned here so the caller attaches to the in-flight work.
 * The two pre-flight conflicts (target folder exists, or a move would overwrite
 * book IDs) are surfaced as {@link FolderTransferConflictError}; any other refusal
 * (a read-only target, an invalid path) propagates with the server's message.
 */
export async function transferFolder(
  sourceFolder: string,
  targetShelfID: string,
  targetFolder: string,
  mode: BookTransferMode
): Promise<string> {
  if (isMockApiMode()) {
    return mockTransferFolder(sourceFolder, targetShelfID, targetFolder, mode);
  }

  const source = foldersFromPath(sourceFolder);
  const target = foldersFromPath(targetFolder);

  try {
    const res = await fetchJson<{ taskchain_id: string }>(
      buildShelfApiPath('/folder-transfers'),
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode, source_folder: source, target_shelf: targetShelfID, target_folder: target })
      }
    );
    return res.taskchain_id;
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const conflict = parseFolderTransferConflict(err.message);
      // The already-running case answers 409 with the chain id, not a conflict
      // body; attach to it rather than failing.
      if (conflict?.taskchain_id) {
        return conflict.taskchain_id;
      }
      if (conflict?.error === 'target_folder_conflict' || conflict?.error === 'book_id_conflict') {
        throw new FolderTransferConflictError(
          conflict.error,
          conflict.message || err.message,
          conflict.conflicting_book_ids ?? []
        );
      }
    }
    throw err;
  }
}

function parseFolderTransferConflict(body: string): FolderTransferConflictBody | null {
  try {
    const parsed = JSON.parse(body) as FolderTransferConflictBody;
    return parsed && typeof parsed === 'object' ? parsed : null;
  } catch {
    // A plain-text 409 (a read-only shelf) is not a conflict body; let the caller
    // propagate the original ApiError so its message reaches the user.
    return null;
  }
}

export async function createFolder(folderPath: string): Promise<void> {
  const normalized = normalizeFolderPath(folderPath);
  if (!normalized) {
    throw new Error('Folder path cannot be empty');
  }

  const encodedPath = encodeFolderPath(normalized);

  if (isMockApiMode()) {
    addMockFolder(normalized);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/folders/${encodedPath}`), {
      method: 'POST'
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      // The server names the reason it refused the name - an empty path, or a
      // hidden/system directory name its scanner would skip - and the user
      // cannot guess between them, so pass the message through instead of
      // replacing it with one fixed guess.
      throw new FolderHttpError(err.message || 'Folder path cannot be empty');
    }

    if (err instanceof ApiError && err.status === 500) {
      throw new FolderHttpError('Failed to create folder');
    }

    const message = err instanceof Error ? err.message : 'Failed to create folder';
    throw new FolderHttpError(message || 'Failed to create folder');
  }
}

export async function renameFolder(folderPath: string, nextName: string): Promise<void> {
  const normalized = normalizeFolderPath(folderPath);
  const name = nextName.trim();
  if (!normalized || normalized === '/' || !name || name.includes('/')) {
    throw new FolderHttpError('Invalid folder name');
  }

  const parentSegments = foldersFromPath(normalized).slice(0, -1);
  const nextPath = [...parentSegments, name].join('/');

  if (isMockApiMode()) {
    renameMockFolder(normalized, nextPath);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/folders/${encodeFolderPath(normalized)}`), {
      method: 'PATCH',
      body: JSON.stringify({ name })
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      throw new FolderHttpError('Invalid folder name');
    }
    if (err instanceof ApiError && err.status === 409) {
      throw new FolderHttpError('Failed to rename folder');
    }
    const message = err instanceof Error ? err.message : 'Failed to rename folder';
    throw new FolderHttpError(message || 'Failed to rename folder');
  }
}

export async function moveFolder(folderPath: string, targetFolderPath: string): Promise<void> {
  const normalized = normalizeFolderPath(folderPath);
  const target = normalizeFolderPath(targetFolderPath);
  if (!normalized || normalized === '/') {
    throw new FolderHttpError('Invalid folder path');
  }
  if (target !== '' && target !== '/' && (target === normalized || target.startsWith(`${normalized}/`))) {
    throw new FolderHttpError('Cannot move a folder under itself.');
  }

  const sourceSegments = foldersFromPath(normalized);
  const folderName = sourceSegments[sourceSegments.length - 1] ?? '';
  const destination = [...foldersFromPath(target), folderName].join('/');

  if (isMockApiMode()) {
    moveMockFolder(normalized, target, destination);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath('/folder-moves'), {
      method: 'POST',
      body: JSON.stringify({ folder: foldersFromPath(normalized), target_folder: foldersFromPath(target) })
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      throw new FolderHttpError('Invalid folder path');
    }
    if (err instanceof ApiError && err.status === 409) {
      throw new FolderHttpError('Failed to move folder');
    }
    const message = err instanceof Error ? err.message : 'Failed to move folder';
    throw new FolderHttpError(message || 'Failed to move folder');
  }
}

export async function deleteFolder(folderPath: string): Promise<void> {
  const normalized = normalizeFolderPath(folderPath);
  if (!normalized || normalized === '/') {
    throw new FolderHttpError('Cannot delete this folder because it is not empty.');
  }

  const encodedPath = encodeFolderPath(normalized);

  if (isMockApiMode()) {
    deleteMockFolder(normalized);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/folders/${encodedPath}`), {
      method: 'DELETE'
    });
  } catch (err) {
    if (err instanceof ApiError && (err.status === 400 || err.status === 409)) {
      throw new FolderHttpError('Cannot delete this folder because it is not empty.');
    }

    const message = err instanceof Error ? err.message : 'Failed to delete folder';
    throw new FolderHttpError(message || 'Failed to delete folder');
  }
}
