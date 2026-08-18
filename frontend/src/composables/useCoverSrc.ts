import { onScopeDispose, ref, watch, type Ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { acquireObjectUrl, releaseObjectUrl } from '@/composables/objectUrlCache';
import bookcover from '@/assets/bookcover.svg';

export interface UseCoverSrcResult {
  src: Ref<string>;
  handleError: () => void;
}

function objectUrlCacheKey(bookId: string, cacheKey: number | undefined): string {
  return `cover::${bookId}::${cacheKey ?? ''}`;
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
 * A provider that reports coversMustBeFetchedAsBlob(): the cover is fetched
 * through the provider and exposed as a `blob:` object URL instead. That is
 * the mobile shell, where a plain `<img src="http://...">` is issued by the
 * WebView directly and bypasses CapacitorHttp — tripping mixed-content
 * blocking against the `https://localhost` origin — and pCloud, whose cover
 * URL is an internal reference rather than a fetchable address. Going through
 * the provider also reuses the offline cover cache for a downloaded book.
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

    const provider = getBookshelfProvider();
    if (!provider.coversMustBeFetchedAsBlob?.()) {
      src.value = isShelfCoverApiUrl(url)
        ? provider.getBookCoverUrl(bookId(), cacheKeyGetter())
        : url;
      return;
    }

    const key = objectUrlCacheKey(bookId(), cacheKeyGetter());
    activeKey = key;

    acquireObjectUrl(key, () => getBookshelfProvider().getBookCover(bookId()))
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
