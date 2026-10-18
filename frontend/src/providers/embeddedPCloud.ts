import { downloadOfflineBook, removeOfflineBook, listOfflineBooks } from '@/api/books';
import { getActiveShelfID, getApiBase } from '@/api/client';
import type { DownloadState } from '@/types/book';
import type { BookshelfReader } from './bookshelfProvider';
import { ShelfVisibility } from './shelfVisibility';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import type { NsfwMarks } from '@/types/book';
import { currentCacheScopeKey } from './cacheScope';
import { LOCAL_API_BASE, LocalShelf, localShelfFetch } from './localShelf';
import type { DavShelfEntry, PCloudShelfEntry } from './mobileConfig';
import { ServerBookshelfProvider } from './serverBookshelfProvider';

/**
 * pCloud and dav entries (another PlainShelf's LAN share, or WebDAV) are read
 * through the embedded Go server (mobile/), using Go's source clients.
 *
 * The API client keeps the entry's `pcloud://` base and shelf root, because
 * those are the cache scope (cacheScope.ts) that downloads, reading progress
 * and history are filed under. Only where a request goes changes: the fetch
 * below sends the entry's shelf routes and server-mode query to Go instead.
 */

/** The Go shelf the active pCloud entry is opened as; one entry is active per app run. */
export const EMBEDDED_PCLOUD_SHELF_ID = 'pcloud';

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
  if (url === `${getApiBase()}/api/mode` || url === `${getApiBase()}/api/shelves`) {
    // Mode is installation-wide and can be read before the source opens.
    // Shelf capabilities must wait until the virtual shelf is registered.
    if (url.endsWith('/api/shelves') && !(await opened(init?.signal ?? (input instanceof Request ? input.signal : undefined)))) {
      throw new TypeError('The virtual shelf is not open in the embedded server.');
    }
    const target = `${LOCAL_API_BASE}${url.slice(getApiBase().length)}`;
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
 * Reads a virtual shelf and its offline copies through Go. The engine applies
 * the device's adult-content choice to both live and downloaded books.
 */
export class EmbeddedPCloudReader implements BookshelfReader {
  private readonly reader = new ServerBookshelfProvider();
  listBooks = this.reader.listBooks.bind(this.reader);
  getBook = this.reader.getBook.bind(this.reader);
  getBookContent = this.reader.getBookContent.bind(this.reader);
  downloadBookContent = this.reader.downloadBookContent.bind(this.reader);
  getReadProgress = this.reader.getReadProgress.bind(this.reader);
  saveReadProgress = this.reader.saveReadProgress.bind(this.reader);
  addReadHistory = this.reader.addReadHistory.bind(this.reader);
  listReadHistoryBooks = this.reader.listReadHistoryBooks.bind(this.reader);
  clearReadHistory = this.reader.clearReadHistory.bind(this.reader);
  getBookCover = this.reader.getBookCover.bind(this.reader);
  getBookCoverUrl = this.reader.getBookCoverUrl.bind(this.reader);
  getDuplicateBookGroups = this.reader.getDuplicateBookGroups.bind(this.reader);
  getSimilarBookPairs = this.reader.getSimilarBookPairs.bind(this.reader);
  getFingerprintStatus = this.reader.getFingerprintStatus.bind(this.reader);
  listTrashedBooks = this.reader.listTrashedBooks.bind(this.reader);
  listFolders = this.reader.listFolders.bind(this.reader);
  listSources = this.reader.listSources.bind(this.reader);
  getSource = this.reader.getSource.bind(this.reader);
  getSourceContent = this.reader.getSourceContent.bind(this.reader);
  getSourceAsset = this.reader.getSourceAsset.bind(this.reader);
  getSourceAssetsBundle = this.reader.getSourceAssetsBundle.bind(this.reader);
  refreshShelf = this.reader.refreshShelf.bind(this.reader);

  downloadBook = downloadOfflineBook;
  removeDownload = removeOfflineBook;
  async listDownloadedBookEntries() {
    const entries = await listOfflineBooks();
    const visibility = new ShelfVisibility({ showNsfw: getShowNsfwOnDevice() });
    return entries.filter((entry) => visibility.allows(entry.book));
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    return (await this.getBook(bookId)).download_state ?? 'not_downloaded';
  }

  filtersNsfwOnDevice(): boolean {
    return true;
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
