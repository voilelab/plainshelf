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


export const EMBEDDED_PCLOUD_SHELF_ID = 'pcloud';

let pushedShowNsfw: boolean | null = null;
let pushing: Promise<void> | null = null;
let opening: Promise<boolean> | null = null;

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

async function syncShowNsfw(): Promise<void> {
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

export async function embeddedPCloudFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const url = requestUrl(input);
  if (url === `${getApiBase()}/api/mode` || url === `${getApiBase()}/api/shelves`) {
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
  if (!(await opened(init?.signal ?? (input instanceof Request ? input.signal : undefined)))) {
    throw new TypeError('The pCloud shelf is not open in the embedded server.');
  }
  await syncShowNsfw();
  const target = `${LOCAL_API_BASE}/api/shelves/${EMBEDDED_PCLOUD_SHELF_ID}${url.slice(prefix.length)}`;
  return localShelfFetch(input instanceof Request ? new Request(target, input) : target, init);
}

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
  listFolderMeta = this.reader.listFolderMeta.bind(this.reader);
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

  async localNsfwMarks(): Promise<ReadonlyMap<string, NsfwMarks> | null> {
    try {
      const { marks } = await LocalShelf.nsfwMarks({ id: EMBEDDED_PCLOUD_SHELF_ID });
      return new Map(Object.entries(marks));
    } catch {
      return null;
    }
  }
}
