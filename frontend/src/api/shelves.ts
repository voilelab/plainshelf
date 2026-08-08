import { buildShelfApiPath, fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';
import { getMobileConnectionConfig } from '@/providers/mobileConfig';

export interface ShelfInfo {
  id: string;
  name: string;
}

/**
 * The single shelf a pCloud connection exposes, or null in any other mode.
 *
 * There is nothing to enumerate: the user names one folder, so the shelf is
 * synthesized from it. Keeping the `{id, name}` shape means the sidebar picker,
 * the active-shelf gate in the layouts and the library's shelf watcher all work
 * unchanged. The id is the folder path, matching what mobileConfig set as the
 * active shelf id so the device-local cache scope agrees.
 */
export function pcloudShelfInfo(): ShelfInfo | null {
  const config = getMobileConnectionConfig();
  if (config?.mode !== 'pcloud' || !config.pcloudShelfRoot) {
    return null;
  }

  const segments = config.pcloudShelfRoot.split('/').filter((segment) => segment.length > 0);
  return {
    id: config.pcloudShelfRoot,
    name: segments[segments.length - 1] ?? config.pcloudShelfRoot
  };
}

const mockShelves: ShelfInfo[] = [
  {
    id: 'default_shelf',
    name: 'Default Shelf'
  }
];

export async function listShelves(): Promise<ShelfInfo[]> {
  // Short-circuited here rather than at each call site so every consumer —
  // the shelves store, the reader layout, the sidebar — gets it for free and
  // none of them issues a request that has no server to answer it.
  const pcloudShelf = pcloudShelfInfo();
  if (pcloudShelf) {
    return [pcloudShelf];
  }

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
