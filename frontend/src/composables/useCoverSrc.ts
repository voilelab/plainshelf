import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { onScopeDispose, ref, watch, type Ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { acquireObjectUrl, releaseObjectUrl } from '@/composables/objectUrlCache';
import bookcover from '@/assets/bookcover.svg';

interface UseCoverSrcResult {
  src: Ref<string>;
  handleError: () => void;
}

function objectUrlCacheKey(bookId: string, cacheKey: number | undefined): string {
  return `cover::${bookId}::${cacheKey ?? ''}`;
}

function isShelfCoverApiUrl(url: string): boolean {
  return url.includes('/api/shelves/') && url.includes('/books/') && url.includes('/cover');
}

// Capacitor Android 8.5.2 patches fetch and XMLHttpRequest for native HTTP, but WebView image loads still use WebViewLocalServer and its network policy.
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
    if (!getPlatformCapabilities().coversMustBeFetchedAsBlob) {
      src.value = isShelfCoverApiUrl(url)
        ? provider.getBookCoverUrl(bookId(), cacheKeyGetter())
        : url;
      return;
    }

    const key = objectUrlCacheKey(bookId(), cacheKeyGetter());
    activeKey = key;

    acquireObjectUrl(key, () => provider.getBookCover(bookId()))
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
