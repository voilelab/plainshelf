import { onBeforeUnmount, ref, type Ref } from 'vue';
import { isMobileRuntime } from '@/providers/runtime';

export const MOBILE_READER_QUERY = '(max-width: 720px)';

export function shouldUseMobileReader(mobileRuntime: boolean, narrowViewport: boolean): boolean {
  return mobileRuntime || narrowViewport;
}

export function useMobileReaderPresentation(): Ref<boolean> {
  const mobileRuntime = isMobileRuntime();
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return ref(mobileRuntime);
  }

  const query = window.matchMedia(MOBILE_READER_QUERY);
  const isMobileReader = ref(shouldUseMobileReader(mobileRuntime, query.matches));
  const onChange = (event: MediaQueryListEvent): void => {
    isMobileReader.value = shouldUseMobileReader(mobileRuntime, event.matches);
  };

  query.addEventListener('change', onChange);
  onBeforeUnmount(() => query.removeEventListener('change', onChange));

  return isMobileReader;
}
