import { toValue, type MaybeRefOrGetter } from 'vue';
import {
  useRoute,
  useRouter,
  type RouteLocationRaw,
  type Router
} from 'vue-router';

interface PlainShelfHistoryState {
  back?: unknown;
}

function isInternalPath(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.startsWith('/') &&
    !value.startsWith('//') &&
    !value.includes('\\')
  );
}

/**
 * Vue Router records the previous in-app fullPath in history.state.back.
 * Only use browser history when that value still resolves to a real
 * PlainShelf route; otherwise history.back() could leave the application.
 */
export function isSafePlainShelfBackTarget(
  router: Router,
  currentFullPath: string,
  candidate: unknown
): candidate is string {
  if (!isInternalPath(candidate) || candidate === currentFullPath) {
    return false;
  }

  const resolved = router.resolve(candidate);
  return resolved.matched.length > 0 && resolved.name !== 'not-found';
}

/**
 * Return to the exact preceding PlainShelf history entry when possible. A
 * direct/deep link has no safe in-app predecessor, so replace it with the
 * caller's fallback instead of adding another history entry.
 */
export function navigateBackSafely(
  router: Router,
  currentFullPath: string,
  fallback: RouteLocationRaw,
  historyState: PlainShelfHistoryState | null = window.history.state as PlainShelfHistoryState | null
): void {
  const back = historyState?.back;
  if (isSafePlainShelfBackTarget(router, currentFullPath, back)) {
    router.back();
    return;
  }

  void router.replace(fallback);
}

export function useSafeBackNavigation(fallback: MaybeRefOrGetter<RouteLocationRaw>) {
  const route = useRoute();
  const router = useRouter();

  function goBack(): void {
    navigateBackSafely(router, route.fullPath, toValue(fallback));
  }

  return { goBack };
}
