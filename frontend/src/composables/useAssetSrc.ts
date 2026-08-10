import { onScopeDispose, ref, watch, type Ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { acquireObjectUrl, releaseObjectUrl } from '@/composables/objectUrlCache';

export interface UseAssetSrcResult {
  /** Empty until the illustration has loaded, and whenever it failed. */
  src: Ref<string>;
  failed: Ref<boolean>;
  handleError: () => void;
}

function objectUrlCacheKey(bookId: string, sourceId: string, name: string): string {
  return `asset::${bookId}::${sourceId}::${name}`;
}

/**
 * Resolves the `src` for one illustration stored in a source's `assets/`
 * directory.
 *
 * Unlike `useCoverSrc`, this fetches through the provider on every runtime
 * rather than handing a URL to the browser on desktop and web. Two reasons: a
 * plain `<img src="...">` carries no API token, so it fails outright when the
 * server sets `protect_read`; and on mobile the WebView issues that request
 * itself, bypassing CapacitorHttp and tripping mixed-content blocking. One code
 * path avoids having a second, platform-shaped way for images to break.
 *
 * A provider that cannot reach assets at all (pCloud today, or an offline
 * mobile cache) simply leaves `failed` set, and the caller shows the alt text.
 */
export function useAssetSrc(
  bookId: () => string,
  sourceId: () => string,
  name: () => string
): UseAssetSrcResult {
  const src = ref('');
  const failed = ref(false);

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
    src.value = '';
    failed.value = false;

    const book = bookId();
    const source = sourceId();
    const assetName = name();
    if (!book || !source || !assetName) {
      failed.value = true;
      return;
    }

    // Bound to the provider that was checked, so the fetch cannot land on a
    // later provider that has no asset support.
    const provider = getBookshelfProvider();
    const fetchAsset = provider.getSourceAsset?.bind(provider);
    if (!fetchAsset) {
      failed.value = true;
      return;
    }

    const key = objectUrlCacheKey(book, source, assetName);
    activeKey = key;

    acquireObjectUrl(key, () => fetchAsset(book, source, assetName))
      .then((objectUrl) => {
        if (token === requestToken) {
          src.value = objectUrl;
        }
      })
      .catch(() => {
        if (token === requestToken) {
          failed.value = true;
        }
      });
  }

  watch([bookId, sourceId, name], resolve, { immediate: true });

  onScopeDispose(releaseActive);

  function handleError(): void {
    failed.value = true;
  }

  return { src, failed, handleError };
}
