import { onBeforeUnmount, ref, type Ref } from 'vue';

// Keep in sync with the @media (max-width: ...) blocks that switch the layout
// into its narrow (drawer) form.
export const NARROW_VIEWPORT_QUERY = '(max-width: 768px)';

/**
 * Reactive match for the narrow-viewport breakpoint. Tracks live viewport
 * resizes, so a desktop window dragged across the breakpoint switches layout
 * without a reload.
 */
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
