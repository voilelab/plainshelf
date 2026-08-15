import { ApiError } from '@/api/client';
import type {
  BookshelfProvider,
  WritableBookshelfProvider
} from './bookshelfProvider';
import { isMobileRuntime, isWailsRuntime } from './runtime';
import {
  getActiveShelfEntry,
  getActiveShelfEntryToken,
  isShelfEntryConfigured
} from './mobileConfig';
import type { ShelfEntry } from './mobileConfig';
import { MobileBookshelfProvider } from './mobileBookshelfProvider';
import { FilesystemMobileBookCache } from './filesystemMobileBookCache';
import { PCloudBookshelfProvider } from './pcloudBookshelfProvider';
import { PCloudClient } from '@/api/pcloud/client';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import { FilesystemShelfSnapshotStore } from './shelfSnapshotStore';
import { WailsBookshelfProvider } from './wailsBookshelfProvider';

let provider: BookshelfProvider | null = null;

/**
 * Picks what the mobile shell reads from, for the shelf the device has active.
 *
 * A pCloud entry is only selected once it is complete. The provider is built
 * during bootstrap, before the router has had a chance to send an unconfigured
 * install to the shelf list, and PCloudClient refuses to construct without a
 * token — so an incomplete entry, or none at all, would throw here and take the
 * whole app down instead of showing the setup form.
 */
function createMobileSource(entry: ShelfEntry | null): BookshelfProvider {
  const accessToken = getActiveShelfEntryToken();
  if (entry?.type !== 'pcloud' || !isShelfEntryConfigured(entry) || !accessToken) {
    return new ServerBookshelfProvider();
  }

  return new PCloudBookshelfProvider({
    client: new PCloudClient({ host: entry.pcloudHost, accessToken }),
    shelfRoot: entry.pcloudShelfRoot,
    // Persisted for the same reason downloads are (see below): without it the
    // shelf would be walked once per app launch, which is what manual updating
    // exists to avoid.
    snapshotStore: new FilesystemShelfSnapshotStore()
  });
}

export function createBookshelfProvider(): BookshelfProvider {
  if (isWailsRuntime()) {
    return new WailsBookshelfProvider();
  }

  if (isMobileRuntime()) {
    // Persist downloads and reading progress across app restarts, in
    // Directory.Data: app-private files are exempt from the WebView's
    // best-effort storage eviction, which the browser storage APIs are not.
    return new MobileBookshelfProvider(
      createMobileSource(getActiveShelfEntry()),
      new FilesystemMobileBookCache()
    );
  }

  return new ServerBookshelfProvider();
}

export function getBookshelfProvider(): BookshelfProvider {
  if (!provider) {
    provider = createBookshelfProvider();
  }
  return provider;
}

/**
 * Mirrors how a read-only server answers a mutation (server/app.go), and how
 * PCloudBookshelfProvider refuses one, so a caller that surfaces the message
 * reads the same either way. Contains "read-only" deliberately: the mobile e2e
 * suite asserts on that phrase.
 */
const WRITES_UNAVAILABLE_MESSAGE = 'This client is read-only. Write operations are disabled.';

/**
 * Whether this provider implements the whole write surface.
 *
 * An intersection is not a union, so `provider.writable === true` does not
 * narrow on its own — the predicate is what does the narrowing, and
 * `implements BookshelfWriter` on the class is what makes it true.
 */
export function isWritableProvider(
  provider: BookshelfProvider
): provider is WritableBookshelfProvider {
  return provider.writable === true;
}

/**
 * The active provider, for a caller that is about to mutate the shelf.
 *
 * Every shelf write goes through here rather than through
 * getBookshelfProvider(), so "this call changes the library" is visible at the
 * call site and there is one place to refuse it. Reads — including the
 * device-local ones on mobile, such as saveReadProgress and the reading
 * history — keep using getBookshelfProvider().
 *
 * Throws rather than rejecting: every call site awaits inside a try, so a
 * synchronous throw is caught wherever this can currently be raised. Do not
 * introduce `bookshelfWriter().x(…).catch(…)` in a non-async context.
 */
export function bookshelfWriter(): WritableBookshelfProvider {
  const active = getBookshelfProvider();
  if (!isWritableProvider(active)) {
    throw new ApiError(WRITES_UNAVAILABLE_MESSAGE, { status: 403 });
  }
  return active;
}

export type {
  BookshelfProvider,
  BookshelfReader,
  BookshelfWriter,
  WritableBookshelfProvider
} from './bookshelfProvider';
export { isMobileRuntime, isWailsRuntime } from './runtime';
export type { CachedBookManifest, MobileBookCache } from './mobileBookCache';
export { InMemoryMobileBookCache } from './mobileBookCache';
export { FilesystemMobileBookCache } from './filesystemMobileBookCache';
