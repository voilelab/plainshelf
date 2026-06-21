import { buildShelfApiPath, fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';

export interface ShelfInfo {
  id: string;
  name: string;
  path: string;
  scan_interval: string;
}

const mockShelves: ShelfInfo[] = [
  {
    id: 'default_shelf',
    name: 'Default Shelf',
    path: '/shelves/default',
    scan_interval: '1m0s'
  }
];

export async function listShelves(): Promise<ShelfInfo[]> {
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

      return [{
        id,
        name,
        path: typeof shelf.path === 'string' ? shelf.path : '',
        scan_interval: typeof shelf.scan_interval === 'string' ? shelf.scan_interval : ''
      }];
    });
}

export async function getShelfStatus(shelfID?: string): Promise<{ ready: boolean }> {
  if (isMockApiMode()) {
    return { ready: true };
  }
  return fetchJson<{ ready: boolean }>(buildShelfApiPath('/status', shelfID));
}

export async function updateShelf(shelfID: string, name: string, scanInterval: string): Promise<void> {
  await fetchJson<void>(`/api/shelves/${encodeURIComponent(shelfID)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, scan_interval: scanInterval })
  });
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
