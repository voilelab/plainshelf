import { getActiveShelfID, getApiBase } from '@/api/client';
import { buildDeviceDocumentKey } from '@/storage/deviceDocument';
import { shelfEntryTarget, type ShelfEntry } from '@/providers/mobileConfig';

export function currentCacheScopeKey(): string {
  return buildDeviceDocumentKey(getApiBase(), getActiveShelfID());
}

export function cacheScopeKeyForEntry(entry: ShelfEntry): string {
  const { apiBase, shelfID } = shelfEntryTarget(entry);
  return buildDeviceDocumentKey(apiBase, shelfID);
}
