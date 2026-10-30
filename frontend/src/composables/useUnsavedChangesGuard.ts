import {
  computed,
  onBeforeUnmount,
  onMounted,
  ref,
  toValue,
  type MaybeRefOrGetter
} from 'vue';
import {
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
  useRouter,
  type RouteLocationNormalized
} from 'vue-router';
import { isMobileRuntime } from '@/providers/runtime';

type LeaveAction = () => void | Promise<unknown>;

interface BrowserHistoryState {
  back?: unknown;
  current?: unknown;
  forward?: unknown;
}

type HistoryTraversalDirection = 'back' | 'forward' | null;

// Vue Router 5.3.1 updates history state on popstate before running navigation guards.
export function historyTraversalDirection(
  toFullPath: string,
  fromFullPath: string,
  state: BrowserHistoryState | null
): HistoryTraversalDirection {
  if (state?.current !== toFullPath) return null;
  if (state.forward === fromFullPath) return 'back';
  if (state.back === fromFullPath) return 'forward';
  return null;
}

interface UnsavedChangesGuardOptions {
  goBack: LeaveAction;
  beforeCheck?: () => void;
}

export function useUnsavedChangesGuard(
  isDirty: MaybeRefOrGetter<boolean>,
  options: UnsavedChangesGuardOptions
) {
  const router = useRouter();
  const pendingLeave = ref<LeaveAction | null>(null);
  const showDiscardConfirmation = computed(() => pendingLeave.value !== null);
  let allowNextRouteLeave = false;
  let mobileBackHandle: { remove: () => Promise<void> } | null = null;
  let unmounted = false;

  function dirtyNow(): boolean {
    options.beforeCheck?.();
    return toValue(isDirty);
  }

  function requestLeave(action: LeaveAction = options.goBack): void {
    if (!dirtyNow()) {
      void action();
      return;
    }
    pendingLeave.value = action;
  }

  function cancelLeave(): void {
    pendingLeave.value = null;
  }

  function confirmLeave(): void {
    const action = pendingLeave.value;
    pendingLeave.value = null;
    if (!action) return;
    allowNextRouteLeave = true;
    void action();
  }

  function leaveWithoutPrompt(action: LeaveAction): void {
    pendingLeave.value = null;
    allowNextRouteLeave = true;
    void action();
  }

  function guardRouteChange(
    to: RouteLocationNormalized,
    from: RouteLocationNormalized
  ): boolean {
    if (allowNextRouteLeave) {
      allowNextRouteLeave = false;
      return true;
    }
    if (!dirtyNow()) return true;

    const state = window.history.state as BrowserHistoryState | null;
    const direction = historyTraversalDirection(to.fullPath, from.fullPath, state);
    if (direction === 'back') {
      pendingLeave.value = options.goBack;
    } else if (direction === 'forward') {
      pendingLeave.value = () => router.forward();
    } else if (state?.back === to.fullPath) {
      pendingLeave.value = options.goBack;
    } else {
      pendingLeave.value = () => router.replace(routeLocationForRetry(to));
    }
    return false;
  }

  onBeforeRouteLeave(guardRouteChange);

  // Vue Router 5.3.1 runs update guards for reused components and leave guards for components being left.
  onBeforeRouteUpdate(guardRouteChange);

  function onBeforeUnload(event: BeforeUnloadEvent): void {
    if (!dirtyNow()) return;
    event.preventDefault();
    event.returnValue = '';
  }

  async function installMobileBackHandler(): Promise<void> {
    if (!isMobileRuntime()) return;
    const { App } = await import('@capacitor/app');
    const handle = await App.addListener('backButton', () => requestLeave());
    if (unmounted) {
      await handle.remove();
      return;
    }
    mobileBackHandle = handle;
  }

  onMounted(() => {
    window.addEventListener('beforeunload', onBeforeUnload);
    void installMobileBackHandler();
  });

  onBeforeUnmount(() => {
    unmounted = true;
    window.removeEventListener('beforeunload', onBeforeUnload);
    void mobileBackHandle?.remove();
    mobileBackHandle = null;
  });

  return {
    showDiscardConfirmation,
    requestLeave,
    cancelLeave,
    confirmLeave,
    leaveWithoutPrompt
  };
}

function routeLocationForRetry(to: RouteLocationNormalized) {
  return {
    path: to.path,
    query: to.query,
    hash: to.hash
  };
}
