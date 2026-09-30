import { getActiveShelfID, getApiBase } from '@/api/client';
import { buildDeviceDocumentKey } from '@/storage/deviceDocument';
import { shelfEntryTarget, type ShelfEntry } from '@/providers/mobileConfig';

/**
 * Identifies the (server, shelf) pair that device-local book data belongs to.
 *
 * A book id is only unique *within one shelf*: shelves written by older servers
 * derive it from md5(folders + "-" + title), so two servers that both call a
 * shelf `default_shelf` really do hand out one id for different books. Anything
 * keyed by book id alone mixes them up, on disk or in RAM.
 *
 * Read fresh at every call site rather than captured once: the connection
 * settings behind it are module-level state the settings UI updates in place,
 * while the provider and cache holding them are process-wide singletons. A
 * caller that must not straddle a connection change captures this value and
 * checks it again itself — see MobileBookshelfProvider.downloadBook.
 */
export function currentCacheScopeKey(): string {
  return buildDeviceDocumentKey(getApiBase(), getActiveShelfID());
}

/**
 * The same key for a shelf entry that is not the active one: deleting an entry
 * has to find its downloads without repointing the live API client at it. Both
 * go through shelfEntryTarget(), because a drift between two derivations would
 * not fail loudly — it would silently strand every downloaded book.
 */
export function cacheScopeKeyForEntry(entry: ShelfEntry): string {
  const { apiBase, shelfID } = shelfEntryTarget(entry);
  return buildDeviceDocumentKey(apiBase, shelfID);
}
