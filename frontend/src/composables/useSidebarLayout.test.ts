// @vitest-environment jsdom

import { createApp, defineComponent, type App } from 'vue';
import { createMemoryHistory, createRouter, type Router } from 'vue-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RAIL_SIDEBAR_WIDTH, useSidebarLayout } from './useSidebarLayout';
import {
  SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY,
  SIDEBAR_MODE_STORAGE_KEY
} from '@/utils/sidebarMode';


function fakePanel(initialWidth: number) {
  const resizes: number[] = [];
  let width = initialWidth;
  return {
    resizes,
    getSize: () => width,
    resize: (next: number) => {
      width = next;
      resizes.push(next);
    }
  };
}

function stubMatchMedia(initialMatches: boolean) {
  const listeners = new Set<(event: MediaQueryListEvent) => void>();
  let matches = initialMatches;

  vi.stubGlobal('matchMedia', (query: string) => ({
    matches,
    media: query,
    addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) =>
      listeners.add(listener),
    removeEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) =>
      listeners.delete(listener)
  }));

  return {
    async set(next: boolean) {
      matches = next;
      for (const listener of listeners) {
        listener({ matches: next } as MediaQueryListEvent);
      }
      await Promise.resolve();
    }
  };
}

let active: { app: App; router: Router } | null = null;

async function mountSidebar(options: { panelWidth?: number; initialPath?: string } = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/books', component: defineComponent({ render: () => null }) },
      { path: '/trash', component: defineComponent({ render: () => null }) }
    ]
  });
  await router.push(options.initialPath ?? '/books');
  await router.isReady();

  const panel = fakePanel(options.panelWidth ?? 240);
  let api!: ReturnType<typeof useSidebarLayout>;
  const Host = defineComponent({
    setup() {
      api = useSidebarLayout();
      return () => null;
    }
  });

  const app = createApp(Host);
  app.use(router);
  app.mount(document.createElement('div'));
  active = { app, router };

  api.sidebarPanelRef.value = panel as unknown as typeof api.sidebarPanelRef.value;
  return { api, router, panel };
}

beforeEach(() => {
  window.localStorage.clear();
  stubMatchMedia(false);
});

afterEach(() => {
  active?.app.unmount();
  active = null;
  vi.unstubAllGlobals();
});

describe('useSidebarLayout mode', () => {
  it('starts expanded and rails on the first toggle', async () => {
    const { api, panel } = await mountSidebar();

    expect(api.isRailSidebar.value).toBe(false);

    await api.toggleSidebarMode();

    expect(api.isRailSidebar.value).toBe(true);
    expect(panel.resizes).toEqual([RAIL_SIDEBAR_WIDTH]);
  });

  it('persists the mode so a reload comes back railed', async () => {
    const { api } = await mountSidebar();
    await api.toggleSidebarMode();
    expect(window.localStorage.getItem(SIDEBAR_MODE_STORAGE_KEY)).toBe('rail');

    active?.app.unmount();
    active = null;
    const reloaded = await mountSidebar();

    expect(reloaded.api.isRailSidebar.value).toBe(true);
  });

  it('restores the width the panel was dragged to before railing', async () => {
    const { api, panel } = await mountSidebar({ panelWidth: 280 });

    await api.toggleSidebarMode();
    expect(window.localStorage.getItem(SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY)).toBe('280');

    await api.toggleSidebarMode();

    expect(api.isRailSidebar.value).toBe(false);
    expect(panel.resizes).toEqual([RAIL_SIDEBAR_WIDTH, 280]);
  });

  it('restores the dragged width across a reload taken while railed', async () => {
    const { api } = await mountSidebar({ panelWidth: 280 });
    await api.toggleSidebarMode();

    active?.app.unmount();
    active = null;
    const reloaded = await mountSidebar({ panelWidth: RAIL_SIDEBAR_WIDTH });
    await reloaded.api.toggleSidebarMode();

    expect(reloaded.panel.resizes).toEqual([280]);
  });

  it('ignores a width below the expanded minimum instead of storing it', async () => {
    const { api } = await mountSidebar({ panelWidth: RAIL_SIDEBAR_WIDTH });

    await api.toggleSidebarMode();

    expect(window.localStorage.getItem(SIDEBAR_EXPANDED_WIDTH_STORAGE_KEY)).toBeNull();
  });
});

describe('useSidebarLayout narrow viewport', () => {
  it('hides the rail nav behind the drawer while the viewport is narrow', async () => {
    const media = stubMatchMedia(false);
    const { api } = await mountSidebar();
    await api.toggleSidebarMode();
    expect(api.showRailNav.value).toBe(true);

    await media.set(true);

    expect(api.isRailSidebar.value).toBe(true);
    expect(api.showRailNav.value).toBe(false);

    await media.set(false);
    expect(api.showRailNav.value).toBe(true);
  });

  it('closes the drawer on navigation and on leaving the narrow viewport', async () => {
    const media = stubMatchMedia(true);
    const { api, router } = await mountSidebar();

    api.drawerOpen.value = true;
    await router.push('/trash');
    expect(api.drawerOpen.value).toBe(false);

    api.drawerOpen.value = true;
    await media.set(false);
    expect(api.drawerOpen.value).toBe(false);
  });
});

describe('useSidebarLayout sections', () => {
  it('collapses and re-expands each foldable section independently', async () => {
    const { api } = await mountSidebar();
    const sections = ['folders', 'reading', 'maintenance', 'admin'] as const;

    for (const section of sections) {
      expect(api.collapsedSidebarSections[section]).toBe(false);
    }

    api.toggleSidebarSection('folders');

    expect(api.collapsedSidebarSections.folders).toBe(true);
    expect(api.collapsedSidebarSections.reading).toBe(false);

    api.toggleSidebarSection('folders');
    expect(api.collapsedSidebarSections.folders).toBe(false);
  });
});
