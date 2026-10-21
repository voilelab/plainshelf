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

// Vue Router 5.3.1 stores the previous fullPath in history.state.back.
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
