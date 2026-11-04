import { Capacitor } from '@capacitor/core';
import { LocalShelf } from '@/providers/localShelf';
import {
  addShelfEntry,
  getShelfEntries,
  newShelfEntry,
  setActiveShelfEntry
} from '@/providers/mobileConfig';

export async function adoptLocalShelves(): Promise<void> {
  if (getShelfEntries().length > 0 || !Capacitor.isNativePlatform()) {
    return;
  }
  try {
    const { shelves } = await LocalShelf.listShelves();
    if (shelves.length === 0) {
      return;
    }
    const adopted = shelves.map((shelf) => ({
      ...newShelfEntry('local'),
      type: 'local' as const,
      name: shelf.name,
      localShelfId: shelf.id
    }));
    for (const entry of adopted) {
      await addShelfEntry(entry);
    }
    await setActiveShelfEntry(adopted[0].id);
  } catch {
    return;
  }
}
