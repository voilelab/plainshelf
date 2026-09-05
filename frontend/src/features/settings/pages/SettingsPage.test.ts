// @vitest-environment jsdom
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createApp, defineComponent, h, nextTick, ref, type App } from 'vue';
import { createMemoryHistory, createRouter, type Router } from 'vue-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

// Two end-to-end cases used to guard this page: one measured every panel's
// offset below the tab list in a real browser, the other counted requests
// across tab switches. Both were really about one thing — Reka keeps an
// inactive tab's wrapper in the DOM, and with `unmount-on-hide` off it keeps
// the content too — so they are checked here, where a tab switch costs
// milliseconds instead of a server and a browser.

/** Counts one entry per panel mount, so a re-mount after a tab switch shows up. */
const mounts = vi.hoisted(() => ({ log: [] as string[] }));

function stubPanel(name: string) {
  return {
    default: defineComponent({
      inheritAttrs: false,
      setup() {
        mounts.log.push(name);
        return () => h('div', { class: 'panel', 'data-panel': name }, name);
      }
    })
  };
}

vi.mock('@/features/settings/components/AboutPanel.vue', () => stubPanel('about'));
vi.mock('@/features/settings/components/CoverPanel.vue', () => stubPanel('cover'));
vi.mock('@/features/settings/components/EpubImportPanel.vue', () => stubPanel('import'));
vi.mock('@/features/settings/components/LanguagePanel.vue', () => stubPanel('language'));
vi.mock('@/features/settings/components/LogRetentionPanel.vue', () => stubPanel('logs'));
vi.mock('@/features/settings/components/NsfwPanel.vue', () => stubPanel('nsfw'));
vi.mock('@/features/settings/components/ReadHistoryPanel.vue', () => stubPanel('read-history'));
vi.mock('@/features/settings/components/ReaderLaunchPanel.vue', () => stubPanel('reader-launch'));
vi.mock('@/features/settings/components/ShelvesPanel.vue', () => stubPanel('shelves'));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref: r } = await import('vue');
  return { useWriteAccess: () => ({ serverSettingsEditable: r(true) }) };
});

// The page loads the server settings once on mount; `loadSettings` is counted
// so a tab switch that re-ran it would be visible.
const form = vi.hoisted(() => ({ loads: 0 }));
vi.mock('@/features/settings/composables/useServerSettingsForm', async () => {
  const { ref: r } = await import('vue');
  return {
    useServerSettingsForm: () => ({
      loading: r(false),
      saving: r(false),
      error: r(''),
      coverToJpg: r(false),
      showNsfw: r(false),
      logRetentionDays: r(30),
      readHistoryLimit: r(50),
      readerLaunchMode: r('tab'),
      epubPreset: r('markdown'),
      epubIncludeDescription: r(false),
      epubImportError: r(''),
      loadSettings: () => {
        form.loads += 1;
        return Promise.resolve();
      },
      onCoverToJpgChange: () => {},
      onShowNsfwChange: () => {},
      onLogRetentionDaysChange: () => {},
      onReadHistoryLimitChange: () => {},
      onReaderLaunchModeChange: () => {},
      onEpubPresetChange: () => {},
      onSaveEpubImportStrategy: () => {}
    })
  };
});

const SettingsPage = (await import('./SettingsPage.vue')).default;

let active: { app: App; host: HTMLElement } | null = null;

async function mountSettings(): Promise<{ host: HTMLElement; router: Router }> {
  mounts.log.length = 0;
  form.loads = 0;

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/settings', component: SettingsPage }]
  });
  await router.push('/settings');
  await router.isReady();

  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(SettingsPage) }));
  app.use(router);
  app.mount(host);
  active = { app, host };
  await nextTick();
  return { host, router };
}

/**
 * Selects a tab by its visible label. Reka activates a trigger from
 * `mousedown.left`, not `click`, so `element.click()` — which dispatches only a
 * click event — leaves the tab list untouched.
 */
async function selectTab(host: HTMLElement, label: string): Promise<void> {
  const tab = [...host.querySelectorAll<HTMLElement>('[role="tab"]')].find(
    (element) => element.textContent?.trim() === label
  );
  if (!tab) {
    throw new Error(`No settings tab labelled ${label}`);
  }
  tab.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }));
  // The active tab is backed by `?tab=`, so the switch is a router navigation:
  // it needs a macrotask to settle, not just a render tick.
  await new Promise((done) => setTimeout(done, 0));
  await nextTick();
}

function panelStates(host: HTMLElement): Record<string, boolean> {
  const states: Record<string, boolean> = {};
  for (const wrapper of host.querySelectorAll<HTMLElement>('.settings-tab-content')) {
    const name = wrapper.querySelector<HTMLElement>('[data-panel]')?.dataset.panel;
    if (name) {
      states[name] = wrapper.hasAttribute('hidden');
    }
  }
  return states;
}

afterEach(() => {
  active?.app.unmount();
  active?.host.remove();
  active = null;
  // The router keeps the tab in ?tab=, so nothing else leaks between cases.
  vi.clearAllMocks();
});

describe('SettingsPage tabs', () => {
  it('mounts every panel once and keeps them mounted across tab switches', async () => {
    const { host } = await mountSettings();

    // Every panel body exists from the first render: with unmountOnHide left at
    // Reka's default only the active tab's content would be here.
    expect(new Set(mounts.log).size).toBe(mounts.log.length);
    expect(mounts.log).toHaveLength(9);

    const mountedOnce = [...mounts.log];
    for (const label of ['About', 'Shelves', 'About', 'Shelves']) {
      await selectTab(host, label);
    }

    // Nothing re-mounted, so a panel's in-flight request or fetched list is
    // still there rather than restarted on every visit.
    expect(mounts.log).toEqual(mountedOnce);
  });

  it('loads the server settings once for the page, not once per tab visit', async () => {
    const { host } = await mountSettings();
    expect(form.loads).toBe(1);

    await selectTab(host, 'About');
    await selectTab(host, 'Shelves');

    expect(form.loads).toBe(1);
  });

  it('marks exactly the inactive panels hidden', async () => {
    const { host } = await mountSettings();

    // Cover is the default tab when the server settings are editable.
    expect(panelStates(host).cover).toBe(false);
    expect(Object.values(panelStates(host)).filter((hidden) => !hidden)).toHaveLength(1);

    await selectTab(host, 'Shelves');

    const states = panelStates(host);
    expect(states.shelves).toBe(false);
    expect(states.cover).toBe(true);
    expect(Object.values(states).filter((hidden) => !hidden)).toHaveLength(1);
  });

  it('collapses a hidden panel, so a retained one takes no layout row', () => {
    // `hidden` above is only half the contract: `.settings-tab-content` is a
    // grid, and an author rule outranks the user-agent `[hidden]` default — so
    // without the paired rule every inactive panel would stay a grid item, its
    // gap would push the active panel down by a different amount per tab, and
    // with unmountOnHide off it would render its content in full. jsdom applies
    // no stylesheet, so the pairing is asserted against the component source.
    // Vitest runs from `frontend/`, and `import.meta.url` is a Vite module id
    // rather than a file URL, so the path is resolved from the root instead.
    const source = readFileSync(
      resolve('src/features/settings/pages/SettingsPage.vue'),
      'utf8'
    );
    const rule = source.match(/\.settings-tab-content\[hidden\]\s*\{([^}]*)\}/);

    expect(rule, '.settings-tab-content[hidden] rule is missing').not.toBeNull();
    expect(rule![1]).toMatch(/display:\s*none/);
  });
});
