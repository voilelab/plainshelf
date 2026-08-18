import { buildShelfApiPath, fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';
import { getShell } from '@/providers/shell';

export interface ShelfInfo {
  id: string;
  name: string;
}

const mockShelves: ShelfInfo[] = [
  {
    id: 'default_shelf',
    name: 'Default Shelf'
  }
];

/**
 * Enumerates the shelves a PlainShelf server offers.
 *
 * Separate from listShelves() because the mobile shelf-entry form has to ask a
 * server the user is still typing in which shelves it has, which is the one
 * place that must not collapse to the active entry.
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

      return [{ id, name }];
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

export async function getShelfStatus(shelfID?: string): Promise<{ ready: boolean }> {
  if (isMockApiMode()) {
    return { ready: true };
  }
  return fetchJson<{ ready: boolean }>(buildShelfApiPath('/status', shelfID));
}

/**
 * Rewrites the shelf's exported book cache now, and reports when the shelf was
 * walked (epoch seconds).
 *
 * The server refreshes that file on its own schedule; this is for a user who
 * has just changed something and does not want to wait before a phone reading
 * the same shelf from cloud storage sees it.
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
