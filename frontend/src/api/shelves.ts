import { fetchJson, getActiveShelfID, isMockApiMode, setActiveShelfID } from './client';

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

export async function listShelves(): Promise<ShelfInfo[]> {
  if (isMockApiMode()) {
    return mockShelves;
  }

  const shelves = await fetchJson<ShelfInfo[]>('/api/shelves');
  if (!Array.isArray(shelves)) {
    return [];
  }

  return shelves.filter((shelf): shelf is ShelfInfo => {
    return Boolean(shelf && typeof shelf.id === 'string' && typeof shelf.name === 'string');
  });
}

export function ensureActiveShelf(shelves: ShelfInfo[]): string {
  const currentShelfID = getActiveShelfID();
  if (shelves.some((shelf) => shelf.id === currentShelfID)) {
    return currentShelfID;
  }

  const fallbackShelfID = shelves[0]?.id ?? 'default_shelf';
  setActiveShelfID(fallbackShelfID);
  return fallbackShelfID;
}
