import { buildShelfApiPath, fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';
import { mockListBooks } from './mocks/books';
import { getMockFolders } from './mocks/folders';
import { getShell } from '@/providers/shell';

interface ShelfCapabilities {
  rescan?: boolean;
  download?: boolean;
  fingerprint?: boolean;
  char_count?: boolean;
}

export interface ShelfInfo {
  kind?: 'local' | 'virtual';
  capabilities?: ShelfCapabilities;
  offline?: { snapshot_at?: string };
  id: string;
  name: string;

  readOnly?: boolean;
}

const mockShelves: ShelfInfo[] = [
  {
    id: 'default_shelf',
    name: 'Default Shelf',
    readOnly: false
  }
];

export async function listServerShelves(): Promise<ShelfInfo[]> {
  if (isMockApiMode()) {
    return mockShelves;
  }

  const shelves = await fetchJson<unknown>('/api/shelves');
  if (!Array.isArray(shelves)) {
    throw new Error('Invalid shelves response from server.');
  }

  return shelves
    .flatMap((shelf): ShelfInfo[] => {
      if (!shelf || typeof shelf.id !== 'string' || typeof shelf.name !== 'string') {
        return [];
      }

      const id = shelf.id.trim();
      const name = shelf.name.trim();
      if (!id || !name) {
        return [];
      }

      const capabilities: ShelfCapabilities = {};
      for (const key of ['rescan', 'download', 'fingerprint', 'char_count'] as const) {
        if (typeof shelf.capabilities?.[key] === 'boolean') capabilities[key] = shelf.capabilities[key];
      }
      return [{
        id, name, readOnly: shelf.read_only === true,
        ...(shelf.kind === 'local' || shelf.kind === 'virtual' ? { kind: shelf.kind } : {}),
        ...(shelf.capabilities && typeof shelf.capabilities === 'object' ? { capabilities } : {}),
        ...(typeof shelf.offline?.snapshot_at === 'string'
          ? { offline: { snapshot_at: shelf.offline.snapshot_at } } : {})
      }];
    });
}

export async function listShelves(): Promise<ShelfInfo[]> {
  const shellShelf = getShell()?.activeShelfInfo?.();
  if (shellShelf) {
    return [await getShell()?.loadActiveShelfInfo?.(shellShelf) ?? shellShelf];
  }

  return listServerShelves();
}

const SCAN_TIMEOUT_MS = 300_000;

interface ShelfScanResult {
  bookCount: number;
  folderCount: number;
}

export class ShelfScanInProgressError extends Error {
  readonly scanID: string;

  constructor(scanID: string) {
    super('A rescan of this shelf is already running.');
    this.name = 'ShelfScanInProgressError';
    this.scanID = scanID;
  }
}

export class ShelfScanRateLimitedError extends Error {
  readonly retryAfterSeconds: number;

  constructor(retryAfterSeconds: number) {
    super('Rescans of this shelf are being requested too quickly.');
    this.name = 'ShelfScanRateLimitedError';
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

export async function rescanShelf(shelfID?: string): Promise<ShelfScanResult> {
  if (isMockApiMode()) {
    return { bookCount: mockListBooks(1, Number.MAX_SAFE_INTEGER).total, folderCount: getMockFolders().length };
  }

  const res = await fetchJson<{
    scan_id?: string;
    book_count?: number;
    folder_count?: number;
    retry_after_seconds?: number;
  }>(
    buildShelfApiPath('/scans', shelfID),
    { method: 'POST' },
    { readOnlySafe: true, acceptStatuses: [409, 429], timeoutMs: SCAN_TIMEOUT_MS }
  );

  if (res?.retry_after_seconds !== undefined) {
    throw new ShelfScanRateLimitedError(res.retry_after_seconds);
  }

  if (res?.book_count === undefined) {
    throw new ShelfScanInProgressError(res?.scan_id ?? '');
  }

  return {
    bookCount: res.book_count,
    folderCount: typeof res.folder_count === 'number' ? res.folder_count : 0
  };
}

export async function exportShelfBookCache(shelfID?: string): Promise<number> {
  if (isMockApiMode()) {
    return Math.floor(Date.now() / 1000);
  }

  const res = await fetchJson<{ timestamp?: number }>(buildShelfApiPath('/book-cache-exports', shelfID), {
    method: 'POST'
  });
  return typeof res?.timestamp === 'number' ? res.timestamp : 0;
}

export function ensureActiveShelf(shelves: ShelfInfo[]): string {
  const currentShelfID = getActiveShelfID();
  if (shelves.some((shelf) => shelf.id === currentShelfID)) {
    return currentShelfID;
  }

  const fallbackShelfID = shelves[0]?.id ?? '';
  setActiveShelfID(fallbackShelfID);
  return fallbackShelfID;
}
