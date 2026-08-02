import { getActiveShelfID, getApiBase } from '@/api/client';
import { buildDeviceDocumentKey } from '@/storage/deviceDocument';

/**
 * Identifies the (server, shelf) pair that device-local book data belongs to.
 *
 * A book id is only unique *within one shelf*: the server seeds it from
 * md5(layers + "-" + title) and only probes for collisions inside that shelf, so
 * two shelves — or two servers that both call a shelf `default_shelf` — hand out
 * the same id for different books. Anything the mobile shell keys by book id
 * alone will therefore mix them up, whether it is stored on the filesystem or
 * memoized in RAM.
 *
 * Shared by FilesystemMobileBookCache (its on-disk scope directory) and
 * MobileBookshelfProvider (its cover object-URL memo) so both agree on what
 * "the same book" means. Deliberately the same key the device-local reading
 * history and reading stats documents use.
 *
 * Read fresh at every call site rather than captured once: the connection
 * settings behind it are module-level state that the settings UI updates in
 * place, while the provider and cache holding them are process-wide singletons.
 * A caller that must not straddle a connection change has to capture this value
 * and check it again itself — see MobileBookshelfProvider.downloadBook.
 */
export function currentCacheScopeKey(): string {
  return buildDeviceDocumentKey(getApiBase(), getActiveShelfID());
}
