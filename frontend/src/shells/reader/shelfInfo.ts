import type { ShelfInfo } from '@/api/shelves';
import { readerBootConfig } from './bootConfig';

export function activeReaderShelfInfo(): ShelfInfo | null {
  const boot = readerBootConfig();
  if (!boot?.shelf_id) {
    return null;
  }

  return { id: boot.shelf_id, name: 'Book', readOnly: false };
}
