import { onBeforeUnmount, ref, type Ref } from 'vue';

export const NARROW_VIEWPORT_QUERY = '(max-width: 768px)';

export function useIsNarrowViewport(): Ref<boolean> {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return ref(false);
  }

  const query = window.matchMedia(NARROW_VIEWPORT_QUERY);
  const isNarrow = ref(query.matches);
  const onChange = (event: MediaQueryListEvent): void => {
    isNarrow.value = event.matches;
  };

  query.addEventListener('change', onChange);
  onBeforeUnmount(() => {
    query.removeEventListener('change', onChange);
  });

  return isNarrow;
}
