import { onScopeDispose, ref, watch, type Ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { acquireObjectUrl, releaseObjectUrl } from '@/composables/objectUrlCache';

interface UseAssetSrcResult {
  src: Ref<string>;
  failed: Ref<boolean>;
  handleError: () => void;
}

function objectUrlCacheKey(bookId: string, sourceId: string, name: string): string {
  return `asset::${bookId}::${sourceId}::${name}`;
}

// HTML image requests cannot attach custom authorization headers and bypass the Capacitor Android 8.5.2 native fetch bridge.
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

    const provider = getBookshelfProvider();
    const fetchAsset = provider.getSourceAsset.bind(provider);

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
