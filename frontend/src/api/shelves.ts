import { buildShelfApiPath, fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';
import {
  getActiveShelfEntry,
  shelfEntryDisplayName,
  shelfEntryTarget
} from '@/providers/mobileConfig';

export interface ShelfInfo {
  id: string;
  name: string;
}

/**
 * The one shelf the mobile shell is pointed at, or null off the mobile shell.
 *
 * The device keeps its own list of shelves — several servers and pCloud folders
 * side by side — and exactly one of them is active. There is nothing here for a
 * server to enumerate: the other entries are not shelves *of this shelf's*
 * server, and a pCloud entry is a folder the user named. So the app-wide shelf
 * list collapses to the active entry, synthesized from it.
 *
 * Keeping the `{id, name}` shape means the sidebar picker, the active-shelf
 * gate in the layouts and the library's shelf watcher all work unchanged. The
 * id is whatever mobileConfig set as the active shelf id — the server's shelf
 * id, or the pCloud folder path — so the device-local cache scope agrees.
 */
export function activeMobileShelfInfo(): ShelfInfo | null {
  const entry = getActiveShelfEntry();
  if (!entry) {
    return null;
  }

  const { shelfID } = shelfEntryTarget(entry);
  if (!shelfID) {
    return null;
  }
  return { id: shelfID, name: shelfEntryDisplayName(entry) };
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
  const mobileShelf = activeMobileShelfInfo();
  if (mobileShelf) {
    return [mobileShelf];
  }

  return listServerShelves();
}

export async function getShelfStatus(shelfID?: string): Promise<{ ready: boolean }> {
  if (isMockApiMode()) {
    return { ready: true };
  }
  return fetchJson<{ ready: boolean }>(buildShelfApiPath('/status', shelfID));
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
