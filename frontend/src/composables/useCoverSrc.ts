import { onScopeDispose, ref, watch, type Ref } from 'vue';
import { getBookshelfProvider } from '../providers';
import { isMobileRuntime } from '../providers/runtime';
import bookcover from '../assets/bookcover.svg';

export interface UseCoverSrcResult {
  src: Ref<string>;
  handleError: () => void;
}

interface ObjectUrlEntry {
  url?: string;
  promise?: Promise<string>;
  refCount: number;
}

// Module-level so object URLs are shared and ref-counted across every
// BookCoverImg instance that ends up showing the same book/cacheKey pair
// (e.g. the same book appearing in both the shelf grid and a "recently
// read" list), not just within a single component instance.
const objectUrlCache = new Map<string, ObjectUrlEntry>();

function objectUrlCacheKey(bookId: string, cacheKey: number | undefined): string {
  return `${bookId}::${cacheKey ?? ''}`;
}

function acquireObjectUrl(bookId: string, cacheKey: number | undefined): { key: string; promise: Promise<string> } {
  const key = objectUrlCacheKey(bookId, cacheKey);
  let entry = objectUrlCache.get(key);
  if (!entry) {
    entry = { refCount: 0 };
    objectUrlCache.set(key, entry);
  }
  entry.refCount += 1;

  if (entry.url) {
    return { key, promise: Promise.resolve(entry.url) };
  }

  if (!entry.promise) {
    entry.promise = getBookshelfProvider()
      .getBookCover(bookId)
      .then((blob) => {
        const url = URL.createObjectURL(blob);
        const current = objectUrlCache.get(key);
        if (current) {
          current.url = url;
        }
        return url;
      })
      .catch((err) => {
        // Do not cache a failed fetch: drop the entry so a later attempt
        // (next mount, or the server becoming reachable again) can retry
        // instead of being stuck replaying the same rejection.
        objectUrlCache.delete(key);
        throw err;
      });
  }

  return { key, promise: entry.promise };
}

function releaseObjectUrl(key: string): void {
  const entry = objectUrlCache.get(key);
  if (!entry) {
    return;
  }
  entry.refCount -= 1;
  if (entry.refCount > 0) {
    return;
  }
  if (entry.url) {
    URL.revokeObjectURL(entry.url);
  }
  objectUrlCache.delete(key);
}

function isShelfCoverApiUrl(url: string): boolean {
  return url.includes('/api/shelves/') && url.includes('/books/') && url.includes('/cover');
}

/**
 * Resolves the `src` to feed a book cover `<img>`.
 *
 * Desktop/web: passthrough of `coverUrl`, exactly as components rendered it
 * before this composable existed. When the URL already looks like the shelf
 * cover API URL it is rewritten through `getBookCoverUrl(bookId, cacheKey)`
 * (same string when `cacheKey` is unset — this only matters for adding
 * cache-busting after a cover upload, see BookCover.vue).
 *
 * Mobile (Capacitor native, or `?mobile-shell-preview=1`): a plain `<img
 * src="http://...">` request is issued by the WebView directly and bypasses
 * CapacitorHttp, which trips mixed-content blocking against the
 * `https://localhost` origin. Fetch the cover through the bookshelf
 * provider instead (which goes through CapacitorHttp on native, and reuses
 * the offline cover cache when the book is downloaded) and expose it as a
 * `blob:` object URL.
 */
export function useCoverSrc(
  bookId: () => string,
  coverUrl: () => string | undefined,
  cacheKey?: () => number | undefined
): UseCoverSrcResult {
  const src = ref<string>(bookcover);
  const cacheKeyGetter = cacheKey ?? ((): number | undefined => undefined);

  let activeKey: string | null = null;
  let requestToken = 0;

  function releaseActive(): void {
    if (activeKey) {
      releaseObjectUrl(activeKey);
      activeKey = null;
    }
  }

  function resolve(): void {
    const token = ++requestToken;
    releaseActive();

    const url = coverUrl();
    if (!url) {
      src.value = bookcover;
      return;
    }

    if (!isMobileRuntime()) {
      src.value = isShelfCoverApiUrl(url)
        ? getBookshelfProvider().getBookCoverUrl(bookId(), cacheKeyGetter())
        : url;
      return;
    }

    const { key, promise } = acquireObjectUrl(bookId(), cacheKeyGetter());
    activeKey = key;

    promise
      .then((objectUrl) => {
        if (token === requestToken) {
          src.value = objectUrl;
        }
      })
      .catch(() => {
        if (token === requestToken) {
          src.value = bookcover;
        }
      });
  }

  watch([bookId, coverUrl, cacheKeyGetter], resolve, { immediate: true });

  onScopeDispose(releaseActive);

  function handleError(): void {
    src.value = bookcover;
  }

  return { src, handleError };
}
