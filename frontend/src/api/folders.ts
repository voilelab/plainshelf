import type { BookTransferMode } from './books';
import { ApiError, buildShelfApiPath, fetchJson, isMockApiMode } from './client';
import { FolderHttpError, FolderMetaConflictError } from './folderErrors';
import { delay } from './mocks/latency';
import {
  addMockFolder,
  deleteMockFolder,
  getMockFolderMeta,
  getMockFolders,
  moveMockFolder,
  renameMockFolder,
  setMockFolderIcon,
  setMockFolderSortKey
} from './mocks/folders';
import { FOLDER_ICON_NAMES } from '@/components/icons/folderIcons';
import { mockTransferFolder } from './mocks/books';
import { normalizeFolderPath } from '@/utils/folders';

export async function setFolderSortKey(
  path: string,
  sort_key: number,
  version: string,
  shelfID?: string
): Promise<void> {
  if (isMockApiMode()) {
    setMockFolderSortKey(path, sort_key, version);
    await delay(undefined);
    return;
  }
  await fetchJson<void>(
    buildShelfApiPath(`/folder-meta/${encodeFolderPath(path)}`, shelfID),
    { method: 'PUT', body: JSON.stringify({ sort_key, version }) }
  );
}

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

type FolderTransferConflictKind = 'target_folder_conflict' | 'book_id_conflict';

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

export class NsfwRevealConfirmationError extends Error {
  constructor(
    readonly hiddenBooks: number,
    message: string
  ) {
    super(message);
    this.name = 'NsfwRevealConfirmationError';
  }
}

export interface FolderChangeOptions {
  confirm?: boolean;
}

const NSFW_REVEAL_CONFLICT = 'nsfw_reveal_requires_confirmation';

function nsfwRevealConfirmation(err: unknown): NsfwRevealConfirmationError | null {
  if (!(err instanceof ApiError) || err.status !== 409) {
    return null;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(err.message);
  } catch {
    return null;
  }

  const body = parsed as { error?: unknown; message?: unknown; hidden_books?: unknown } | null;
  if (!body || body.error !== NSFW_REVEAL_CONFLICT) {
    return null;
  }

  return new NsfwRevealConfirmationError(
    typeof body.hidden_books === 'number' ? body.hidden_books : 0,
    typeof body.message === 'string' ? body.message : err.message
  );
}

function withConfirm(path: string, confirm?: boolean): string {
  return confirm ? `${path}?confirm=1` : path;
}

interface FolderTransferConflictBody {
  error: FolderTransferConflictKind;
  message?: string;
  conflicting_book_ids: string[];
}

interface FolderTransferBusyBody {
  taskchain_id?: string;
}

type FolderTransfer409Body = Partial<FolderTransferConflictBody> & FolderTransferBusyBody;

function isFolderTransferConflict(
  body: FolderTransfer409Body
): body is FolderTransferConflictBody {
  return (
    (body.error === 'target_folder_conflict' || body.error === 'book_id_conflict') &&
    Array.isArray(body.conflicting_book_ids)
  );
}

export async function transferFolder(
  sourceFolder: string,
  targetShelfID: string,
  targetFolder: string,
  mode: BookTransferMode,
  options?: FolderChangeOptions
): Promise<string> {
  if (isMockApiMode()) {
    return mockTransferFolder(sourceFolder, targetShelfID, targetFolder, mode);
  }

  const source = foldersFromPath(sourceFolder);
  const target = foldersFromPath(targetFolder);

  try {
    const res = await fetchJson<{ taskchain_id: string }>(
      withConfirm(buildShelfApiPath('/folder-transfers'), options?.confirm),
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode, source_folder: source, target_shelf: targetShelfID, target_folder: target })
      }
    );
    return res.taskchain_id;
  } catch (err) {
    const reveal = nsfwRevealConfirmation(err);
    if (reveal) {
      throw reveal;
    }
    if (err instanceof ApiError && err.status === 409) {
      const conflict = parseFolderTransferConflict(err.message);
      if (conflict?.taskchain_id) {
        return conflict.taskchain_id;
      }
      if (conflict && isFolderTransferConflict(conflict)) {
        throw new FolderTransferConflictError(
          conflict.error,
          conflict.message || err.message,
          conflict.conflicting_book_ids
        );
      }
    }
    throw err;
  }
}

function parseFolderTransferConflict(body: string): FolderTransfer409Body | null {
  try {
    const parsed = JSON.parse(body) as FolderTransfer409Body;
    return parsed && typeof parsed === 'object' ? parsed : null;
  } catch {
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
      throw new FolderHttpError(err.message || 'Folder path cannot be empty');
    }

    if (err instanceof ApiError && err.status === 500) {
      throw new FolderHttpError('Failed to create folder');
    }

    const message = err instanceof Error ? err.message : 'Failed to create folder';
    throw new FolderHttpError(message || 'Failed to create folder');
  }
}

export async function renameFolder(
  folderPath: string,
  nextName: string,
  options?: FolderChangeOptions
): Promise<void> {
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
    await fetchJson<void>(
      withConfirm(buildShelfApiPath(`/folders/${encodeFolderPath(normalized)}`), options?.confirm),
      {
        method: 'PATCH',
        body: JSON.stringify({ name })
      }
    );
  } catch (err) {
    const reveal = nsfwRevealConfirmation(err);
    if (reveal) {
      throw reveal;
    }
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

export async function moveFolder(
  folderPath: string,
  targetFolderPath: string,
  options?: FolderChangeOptions
): Promise<void> {
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
    await fetchJson<void>(withConfirm(buildShelfApiPath('/folder-moves'), options?.confirm), {
      method: 'POST',
      body: JSON.stringify({ folder: foldersFromPath(normalized), target_folder: foldersFromPath(target) })
    });
  } catch (err) {
    const reveal = nsfwRevealConfirmation(err);
    if (reveal) {
      throw reveal;
    }
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

export interface FolderMeta {
  sort_key?: number;
  icon: string;
  version: string;
  readOnly: boolean;
}

export interface FolderMetaListing {
  icons: string[];
  folders: Record<string, FolderMeta>;
}

function parseFolderMeta(value: unknown): FolderMeta | null {
  if (typeof value !== 'object' || value === null) {
    return null;
  }
  const raw = value as Record<string, unknown>;
  return {
    icon: typeof raw.icon === 'string' ? raw.icon : '',
    version: typeof raw.version === 'string' ? raw.version : '',
    readOnly: raw.read_only === true,
    ...(typeof raw.sort_key === 'number' && Number.isFinite(raw.sort_key) ? {sort_key: raw.sort_key} : {})
  };
}

export async function getFolderMeta(shelfID?: string): Promise<FolderMetaListing> {
  if (isMockApiMode()) {
    return delay({ icons: [...FOLDER_ICON_NAMES], folders: getMockFolderMeta() });
  }

  const data = await fetchJson<unknown>(buildShelfApiPath('/folder-meta', shelfID), { method: 'GET' });
  const raw = (typeof data === 'object' && data !== null ? data : {}) as Record<string, unknown>;

  const icons = Array.isArray(raw.icons) ? raw.icons.filter((icon): icon is string => typeof icon === 'string') : [];
  const folders: Record<string, FolderMeta> = {};
  if (typeof raw.folders === 'object' && raw.folders !== null) {
    for (const [path, value] of Object.entries(raw.folders as Record<string, unknown>)) {
      const meta = parseFolderMeta(value);
      const normalized = normalizeFolderPath(path);
      if (meta && normalized) {
        folders[normalized] = meta;
      }
    }
  }
  return { icons, folders };
}

export async function setFolderIcon(folderPath: string, icon: string, version: string): Promise<void> {
  const normalized = normalizeFolderPath(folderPath);
  if (!normalized) {
    throw new FolderHttpError('Invalid folder path');
  }

  if (isMockApiMode()) {
    setMockFolderIcon(normalized, icon, version);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/folder-meta/${encodeFolderPath(normalized)}`), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ icon, version })
    });
  } catch (err) {
    if (err instanceof ApiError && err.code === 'FOLDER_META_CONFLICT') {
      throw new FolderMetaConflictError(err.message);
    }
    throw err;
  }
}
