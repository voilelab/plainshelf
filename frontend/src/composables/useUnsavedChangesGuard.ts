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

export type HistoryTraversalDirection = 'back' | 'forward' | null;

/**
 * Vue Router exposes the entry reached by a popstate event while leave guards
 * run. That lets a cancelled browser/mouse history action be retried in the
 * same direction after the user confirms, instead of replacing the editor and
 * leaving a duplicate entry behind it.
 */
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
  /** The shared safe-back action used by the editor's button and native back. */
  goBack: LeaveAction;
  /** Flushes any editor buffer whose reactive dirty flag trails the visible text. */
  beforeCheck?: () => void;
}

/**
 * Protects every way out of an editor with one confirmation state:
 * in-page back, Vue Router navigation, browser/mouse history, page unload and
 * Capacitor's Android system back button.
 */
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
      // A sidebar/link can point at the same entry the shared back action
      // would consume. Use that entry instead of replacing the editor with a
      // duplicate copy of it.
      pendingLeave.value = options.goBack;
    } else {
      // A link/router navigation has already been cancelled. Replacing the
      // editor entry honours its intended destination without adding history.
      pendingLeave.value = () => router.replace(routeLocationForRetry(to));
    }
    return false;
  }

  onBeforeRouteLeave(guardRouteChange);

  // A parameter-only move between two entries of the same route — /books/A to
  // /books/B, reached by a link or by browser history — reuses this component,
  // so Vue Router runs the update guard and never the leave guard above. The
  // page would then swap books underneath the open editor and discard the
  // draft without asking. Both guards share one decision.
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
