import { buildShelfApiPath, fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';
import { mockListBooks } from './mocks/books';
import { getMockFolders } from './mocks/folders';
import { getShell } from '@/providers/shell';

export interface ShelfInfo {
  id: string;
  name: string;

  /**
   * Whether this shelf refuses writes. It exists so the UI can drop the write
   * affordances a read-only shelf has no use for; the server still answers 409
   * either way, so this is never the gate.
   *
   * Absent means writable: an older server that does not send the field is the
   * one that had no read-only shelves to describe.
   */
  readOnly?: boolean;
}

const mockShelves: ShelfInfo[] = [
  {
    id: 'default_shelf',
    name: 'Default Shelf',
    readOnly: false
  }
];

/**
 * Separate from listShelves() because the mobile shelf-entry form has to ask a
 * server the user is still typing in which shelves it has — the one place that
 * must not collapse to the active entry.
 */
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

      // Defaulted rather than required: a server predating the field describes
      // shelves it never opened read-only, and treating those as read-only
      // would hide every write button against a shelf that accepts writes.
      return [{ id, name, readOnly: shelf.read_only === true }];
    });
}

export async function listShelves(): Promise<ShelfInfo[]> {
  // Short-circuited here rather than at each call site so every consumer — the
  // shelves store, the reader layout, the sidebar — gets it for free and none
  // of them issues a request that has no server to answer it.
  const shellShelf = getShell()?.activeShelfInfo?.();
  if (shellShelf) {
    return [shellShelf];
  }

  return listServerShelves();
}

// A full walk of a shelf can take minutes on an SMB or cloud mount, which is
// exactly where this button is most needed, so it gets the streaming timeout
// rather than the metadata one.
const SCAN_TIMEOUT_MS = 300_000;

/**
 * What one shelf rescan found.
 */
interface ShelfScanResult {
  bookCount: number;
  folderCount: number;
}

/**
 * Thrown when the shelf is already being rescanned, so this request was refused
 * rather than started alongside it. `scanID` names the walk already running.
 */
export class ShelfScanInProgressError extends Error {
  readonly scanID: string;

  constructor(scanID: string) {
    super('A rescan of this shelf is already running.');
    this.name = 'ShelfScanInProgressError';
    this.scanID = scanID;
  }
}

/**
 * Thrown when this client asked for walks faster than the server performs them.
 * Distinct from {@link ShelfScanInProgressError}, which ends on its own: this
 * one is about the pace of the asking.
 */
export class ShelfScanRateLimitedError extends Error {
  readonly retryAfterSeconds: number;

  constructor(retryAfterSeconds: number) {
    super('Rescans of this shelf are being requested too quickly.');
    this.name = 'ShelfScanRateLimitedError';
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

/**
 * The answer to "I put a book in the folder and it is not there": the server
 * discovers external changes on its own only every `scan_interval`, and an SMB
 * or cloud mount sends no change notification.
 *
 * A POST that writes nothing, hence `readOnlySafe`.
 */
export async function rescanShelf(shelfID?: string): Promise<ShelfScanResult> {
  if (isMockApiMode()) {
    // Reports the fixture set, so the button says the same thing the grid
    // beside it shows rather than "found nothing".
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

  // Checked before the counts, because the 429 body has none either and would
  // otherwise be reported as a walk that is running when none is.
  if (res?.retry_after_seconds !== undefined) {
    throw new ShelfScanRateLimitedError(res.retry_after_seconds);
  }

  // The 409 body carries only the running walk's ID; the counts are absent
  // because they belong to a walk this request did not perform.
  if (res?.book_count === undefined) {
    throw new ShelfScanInProgressError(res?.scan_id ?? '');
  }

  return {
    bookCount: res.book_count,
    folderCount: typeof res.folder_count === 'number' ? res.folder_count : 0
  };
}

/**
 * Rewrites the exported book cache now, reporting when the shelf was walked
 * (epoch seconds). The server refreshes it on its own schedule; this is for a
 * user who does not want to wait before a phone reading the same shelf from
 * cloud storage sees the change.
 */
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
