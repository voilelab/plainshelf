import { getActiveShelfID, getApiBase } from '@/api/client';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import type { NsfwMarks } from '@/types/book';
import { currentCacheScopeKey } from './cacheScope';
import { LOCAL_API_BASE, LocalShelf, localShelfFetch } from './localShelf';
import type { DavShelfEntry, PCloudShelfEntry } from './mobileConfig';
import { ServerBookshelfProvider } from './serverBookshelfProvider';

/**
 * A pCloud shelf read through the embedded Go server (mobile/) instead of the
 * TypeScript reader in api/pcloud. A dav entry (another PlainShelf's LAN share,
 * or WebDAV) is read the same way, through Go's own source client.
 *
 * The API client keeps the entry's `pcloud://` base and shelf root, because
 * those are the cache scope (cacheScope.ts) that downloads, reading progress
 * and history are filed under. Only where a request goes changes: the fetch
 * below sends the entry's shelf routes and server-mode query to Go instead.
 */

/** The Go shelf the active pCloud entry is opened as; one entry is active per app run. */
const EMBEDDED_PCLOUD_SHELF_ID = 'pcloud';

/** The device choice Go was last given, or null before the shelf is open. */
let pushedShowNsfw: boolean | null = null;
let pushing: Promise<void> | null = null;
/** The open in progress or done: true once the shelf is open. */
let opening: Promise<boolean> | null = null;

/**
 * Opens the entry in Go, with the device's current adult-content choice.
 * Not awaited at startup: opening reads the source, which on a network that
 * hangs rather than fails takes as long as its timeouts. Requests wait for it
 * instead, within their own time budget.
 */
export function openEmbeddedPCloudShelf(entry: PCloudShelfEntry | DavShelfEntry): Promise<boolean> {
  const showNsfw = getShowNsfwOnDevice();
  const open =
    entry.type === 'dav'
      ? LocalShelf.openSourceShelf({
          id: EMBEDDED_PCLOUD_SHELF_ID,
          entryId: entry.id,
          name: entry.name,
          source: entry.davSource,
          url: entry.davUrl,
          username: entry.davUsername ?? '',
          showNsfw
        })
      : LocalShelf.openPCloudShelf({
          id: EMBEDDED_PCLOUD_SHELF_ID,
          entryId: entry.id,
          name: entry.name,
          host: entry.pcloudHost,
          folder: entry.pcloudShelfRoot,
          scope: currentCacheScopeKey(),
          showNsfw
        });
  opening = open.then(
    () => {
      pushedShowNsfw = showNsfw;
      return true;
    },
    (err: unknown) => {
      console.error('Could not open the virtual shelf in the embedded server.', err);
      return false;
    }
  );
  return opening;
}

/** Waits for the open, giving up when the request is aborted. */
function opened(signal: AbortSignal | null | undefined): Promise<boolean> {
  if (!opening) {
    return Promise.resolve(false);
  }
  if (!signal) {
    return opening;
  }
  signal.throwIfAborted();
  return new Promise((resolve, reject) => {
    const onAbort = () => reject(signal.reason);
    signal.addEventListener('abort', onAbort, { once: true });
    opening!.then(resolve).finally(() => signal.removeEventListener('abort', onAbort));
  });
}

/**
 * Hands Go the device choice when it has changed since. A failed push rejects
 * the request rather than let it through: Go would answer with the old choice,
 * which may be the one that shows.
 */
async function syncShowNsfw(): Promise<void> {
  // Compared again after every push: the setting can change while one is on
  // the bridge, and a request let through then would get the old answer.
  for (;;) {
    while (pushing) {
      await pushing.catch(() => undefined);
    }
    const show = getShowNsfwOnDevice();
    if (show === pushedShowNsfw) {
      return;
    }
    const push = LocalShelf.setShelfShowNsfw({ id: EMBEDDED_PCLOUD_SHELF_ID, show }).then(() => {
      pushedShowNsfw = show;
    });
    pushing = push;
    try {
      await push;
    } finally {
      if (pushing === push) {
        pushing = null;
      }
    }
  }
}

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

/**
 * The API client's fetch while a pCloud entry is active. The entry's shelf
 * routes go to the Go shelf, and /api/mode asks the embedded server. Anything
 * else is left alone and fails against the `pcloud://` base.
 */
export async function embeddedPCloudFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const url = requestUrl(input);
  if (url === `${getApiBase()}/api/mode`) {
    // Installation-wide: independent of opening the cloud shelf or its NSFW
    // choice. A writable server does not grant this reader a write surface.
    const target = `${LOCAL_API_BASE}/api/mode`;
    return localShelfFetch(input instanceof Request ? new Request(target, input) : target, init);
  }
  const prefix = `${getApiBase()}/api/shelves/${encodeURIComponent(getActiveShelfID())}`;
  if (url !== prefix && !url.startsWith(`${prefix}/`) && !url.startsWith(`${prefix}?`)) {
    return fetch(input, init);
  }
  // Thrown rather than answered: like no network at all, so a download still
  // opens when the shelf could not be.
  if (!(await opened(init?.signal ?? (input instanceof Request ? input.signal : undefined)))) {
    throw new TypeError('The pCloud shelf is not open in the embedded server.');
  }
  await syncShowNsfw();
  const target = `${LOCAL_API_BASE}/api/shelves/${EMBEDDED_PCLOUD_SHELF_ID}${url.slice(prefix.length)}`;
  return localShelfFetch(input instanceof Request ? new Request(target, input) : target, init);
}

/**
 * The reader MobileBookshelfProvider wraps for a pCloud entry. The adult-content
 * choice stays the device's, as it was with the TypeScript reader: Go applies
 * it, and the offline cache applies it again to downloads.
 */
export class EmbeddedPCloudReader extends ServerBookshelfProvider {
  filtersNsfwOnDevice(): boolean {
    return true;
  }

  /** Go would read every book's source meta.json over pCloud for one listing. */
  supportsCharCountListing(): boolean {
    return false;
  }

  /** From Go, which holds them for every book, hidden or not. */
  async localNsfwMarks(): Promise<ReadonlyMap<string, NsfwMarks> | null> {
    try {
      const { marks } = await LocalShelf.nsfwMarks({ id: EMBEDDED_PCLOUD_SHELF_ID });
      return new Map(Object.entries(marks));
    } catch {
      return null;
    }
  }
}
