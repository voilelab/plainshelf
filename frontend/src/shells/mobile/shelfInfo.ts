import type { ShelfInfo } from '@/api/shelves';
import {
  getActiveShelfEntry,
  shelfEntryDisplayName,
  shelfEntryTarget
} from '@/providers/mobileConfig';

/**
 * The one shelf this device is pointed at, or null when none is configured yet.
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
  // A pCloud entry is a storage backend this app only ever reads, so it reports
  // the same read-only flag a server-side read-only shelf does. The mobile
  // provider is a reading client whatever it is pointed at, so `platform`
  // already covers the write gate — this just keeps the two ways of being
  // read-only from disagreeing about the same shelf.
  return {
    id: shelfID,
    name: shelfEntryDisplayName(entry),
    readOnly: entry.type === 'pcloud'
  };
}
