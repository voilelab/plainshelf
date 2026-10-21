import type { ShelfInfo } from '@/api/shelves';
import {
  getActiveShelfEntry,
  shelfEntryDisplayName,
  shelfEntryTarget
} from '@/providers/mobileConfig';

export function activeMobileShelfInfo(): ShelfInfo | null {
  const entry = getActiveShelfEntry();
  if (!entry) {
    return null;
  }

  const { shelfID } = shelfEntryTarget(entry);
  if (!shelfID) {
    return null;
  }
  return { id: shelfID, name: shelfEntryDisplayName(entry), readOnly: false };
}
