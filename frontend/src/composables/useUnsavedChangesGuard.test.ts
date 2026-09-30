// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, ref, type App, type Ref } from 'vue';
import {
  createMemoryHistory,
  createRouter,
  RouterView,
  type Router
} from 'vue-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

const mobile = vi.hoisted(() => ({
  isMobileRuntime: vi.fn(() => false),
  addListener: vi.fn(),
  backHandler: null as null | (() => void),
  remove: vi.fn().mockResolvedValue(undefined)
}));

vi.mock('@/providers/runtime', () => ({
  isMobileRuntime: mobile.isMobileRuntime
}));

vi.mock('@capacitor/app', () => ({
  App: {
    addListener: mobile.addListener
  }
}));
import {
  historyTraversalDirection,
  useUnsavedChangesGuard
} from './useUnsavedChangesGuard';

interface GuardControls {
  showDiscardConfirmation: Readonly<Ref<boolean>>;
  requestLeave: (action?: () => void | Promise<unknown>) => void;
  cancelLeave: () => void;
  confirmLeave: () => void;
}

let mounted: { app: App; host: HTMLElement } | null = null;

async function mountGuard(
  dirty: Ref<boolean>,
  goBack = vi.fn(),
  beforeCheck?: () => void,
  startPath = '/editor'
) {
  let controls: GuardControls | null = null;
  const Guarded = defineComponent({
    setup() {
      controls = useUnsavedChangesGuard(dirty, { goBack, beforeCheck });
      return () => h('main', 'editor');
    }
  });
  const Other = defineComponent({ setup: () => () => h('main', 'other') });
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/editor', component: Guarded },
      // Same route record for every id, so moving between two books reuses the
      // guarded component instead of remounting it.
      { path: '/books/:id', component: Guarded },
      { path: '/other', component: Other }
    ]
  });
  await router.push(startPath);

  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(RouterView) });
  app.use(router);
  app.mount(host);
  await router.isReady();
  mounted = { app, host };

  if (!controls) throw new Error('guard controls were not mounted');
  return { controls: controls as GuardControls, router, goBack };
}

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  mobile.isMobileRuntime.mockReturnValue(false);
  mobile.addListener.mockReset();
  mobile.backHandler = null;
  mobile.remove.mockClear();
});

describe('history traversal direction', () => {
  it('recognises browser and mouse back without treating a normal route push as history', () => {
    expect(historyTraversalDirection('/books', '/books/1/sources', {
      current: '/books',
      forward: '/books/1/sources'
    })).toBe('back');
    expect(historyTraversalDirection('/books/1', '/books/1/sources', {
      current: '/books/1',
      back: '/books/1/sources'
    })).toBe('forward');
    expect(historyTraversalDirection('/books', '/books/1/sources', {
      current: '/books/1/sources',
      back: '/books'
    })).toBeNull();
  });
});

describe('unsaved changes guard', () => {
  it('leaves clean editors directly', async () => {
    const goBack = vi.fn();
    const { controls } = await mountGuard(ref(false), goBack);

    controls.requestLeave();

    expect(goBack).toHaveBeenCalledOnce();
    expect(controls.showDiscardConfirmation.value).toBe(false);
  });

  it('keeps the dirty editor intact on cancel and runs the pending action on confirm', async () => {
    const dirty = ref(true);
    const goBack = vi.fn();
    const { controls } = await mountGuard(dirty, goBack);

    controls.requestLeave();
    expect(controls.showDiscardConfirmation.value).toBe(true);
    expect(goBack).not.toHaveBeenCalled();

    controls.cancelLeave();
    expect(controls.showDiscardConfirmation.value).toBe(false);
    expect(dirty.value).toBe(true);

    controls.requestLeave();
    controls.confirmLeave();
    expect(goBack).toHaveBeenCalledOnce();
    expect(controls.showDiscardConfirmation.value).toBe(false);
  });

  it('flushes a buffered editor change before deciding whether to leave', async () => {
    const dirty = ref(false);
    const goBack = vi.fn();
    const flush = vi.fn(() => { dirty.value = true; });
    const { controls } = await mountGuard(dirty, goBack, flush);

    controls.requestLeave();

    expect(flush).toHaveBeenCalled();
    expect(goBack).not.toHaveBeenCalled();
    expect(controls.showDiscardConfirmation.value).toBe(true);
  });

  it('cancels a dirty route navigation, then replaces the editor entry after confirmation', async () => {
    const dirty = ref(true);
    const { controls, router } = await mountGuard(dirty);
    const replace = vi.spyOn(router, 'replace');

    await router.push('/other');
    await nextTick();
    expect(router.currentRoute.value.fullPath).toBe('/editor');
    expect(controls.showDiscardConfirmation.value).toBe(true);

    controls.confirmLeave();
    await vi.waitFor(() => expect(router.currentRoute.value.fullPath).toBe('/other'));
    expect(replace).toHaveBeenCalledWith({ path: '/other', query: {}, hash: '' });
  });

  it('uses safe back when a link targets the existing previous entry', async () => {
    const dirty = ref(true);
    const goBack = vi.fn();
    const { controls, router } = await mountGuard(dirty, goBack);
    window.history.replaceState(
      { current: '/editor', back: '/other', forward: null },
      '',
      '/editor'
    );

    await router.push('/other');
    expect(router.currentRoute.value.fullPath).toBe('/editor');

    controls.confirmLeave();
    expect(goBack).toHaveBeenCalledOnce();
  });

  it('guards a parameter-only move between two entries of the same route', async () => {
    // Vue Router reuses the component here, so this navigation reaches the
    // update guard and never the leave guard — the case that used to swap the
    // book underneath a dirty editor without asking.
    const dirty = ref(true);
    const { controls, router } = await mountGuard(dirty, vi.fn(), undefined, '/books/1');
    const replace = vi.spyOn(router, 'replace');

    await router.push('/books/2');
    await nextTick();
    expect(router.currentRoute.value.fullPath).toBe('/books/1');
    expect(controls.showDiscardConfirmation.value).toBe(true);

    controls.confirmLeave();
    await vi.waitFor(() => expect(router.currentRoute.value.fullPath).toBe('/books/2'));
    expect(replace).toHaveBeenCalledWith({ path: '/books/2', query: {}, hash: '' });
  });

  it('requests the browser-native unload warning only while dirty', async () => {
    const dirty = ref(false);
    await mountGuard(dirty);

    const cleanEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(cleanEvent);
    expect(cleanEvent.defaultPrevented).toBe(false);

    dirty.value = true;
    const dirtyEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(dirtyEvent);
    expect(dirtyEvent.defaultPrevented).toBe(true);
  });

  it('routes Android system back through the same dirty confirmation', async () => {
    mobile.isMobileRuntime.mockReturnValue(true);
    mobile.addListener.mockImplementation(async (_event: string, handler: () => void) => {
      mobile.backHandler = handler;
      return { remove: mobile.remove };
    });
    const goBack = vi.fn();
    const { controls } = await mountGuard(ref(true), goBack);
    await vi.waitFor(() => expect(mobile.backHandler).not.toBeNull());

    mobile.backHandler?.();
    expect(controls.showDiscardConfirmation.value).toBe(true);
    expect(goBack).not.toHaveBeenCalled();

    controls.confirmLeave();
    expect(goBack).toHaveBeenCalledOnce();
  });
});
