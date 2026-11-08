// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, ref, type App } from 'vue';
import { createMemoryHistory, createRouter, type Router } from 'vue-router';
import { afterEach, describe, expect, it, vi } from 'vitest';


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
vi.mock('@/features/settings/components/DemoStoragePanel.vue', () => stubPanel('demo-storage'));
vi.mock('@/features/settings/components/EpubImportPanel.vue', () => stubPanel('import'));
vi.mock('@/features/settings/components/LanguagePanel.vue', () => stubPanel('language'));
vi.mock('@/features/settings/components/LogRetentionPanel.vue', () => stubPanel('logs'));
vi.mock('@/features/settings/components/NsfwPanel.vue', () => stubPanel('nsfw'));
vi.mock('@/features/settings/components/ReadHistoryPanel.vue', () => stubPanel('read-history'));
vi.mock('@/features/settings/components/ReaderLaunchPanel.vue', () => stubPanel('reader-launch'));
vi.mock('@/features/settings/components/ShelvesPanel.vue', () => stubPanel('shelves'));
vi.mock('@/features/settings/components/ThemePanel.vue', () => stubPanel('theme'));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref: r } = await import('vue');
  return { useWriteAccess: () => ({ serverSettingsEditable: r(true) }) };
});

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

// Reka UI 2.10.5 TabsTrigger activates on mousedown.left; a synthetic click alone does not activate it.
async function selectTab(host: HTMLElement, label: string): Promise<void> {
  const tab = [...host.querySelectorAll<HTMLElement>('[role="tab"]')].find(
    (element) => element.textContent?.trim() === label
  );
  if (!tab) {
    throw new Error(`No settings tab labelled ${label}`);
  }
  tab.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }));
  await new Promise((done) => setTimeout(done, 0));
  await nextTick();
}

function panelsByTab(host: HTMLElement): { visible: string[][]; hidden: string[][] } {
  const visible: string[][] = [];
  const hidden: string[][] = [];
  for (const wrapper of host.querySelectorAll<HTMLElement>('.settings-tab-content')) {
    const names = [...wrapper.querySelectorAll<HTMLElement>('[data-panel]')].map(
      (panel) => panel.dataset.panel ?? ''
    );
    (wrapper.hasAttribute('hidden') ? hidden : visible).push(names);
  }
  return { visible, hidden };
}

afterEach(() => {
  active?.app.unmount();
  active?.host.remove();
  active = null;
  delete window.plainshelfDemo;
  vi.clearAllMocks();
});

describe('SettingsPage tabs', () => {
  it('mounts every panel once and keeps them mounted across tab switches', async () => {
    const { host } = await mountSettings();

    expect(new Set(mounts.log).size).toBe(mounts.log.length);
    expect(mounts.log).toHaveLength(10);

    const mountedOnce = [...mounts.log];
    for (const label of ['System', 'Shelves', 'System', 'Shelves']) {
      await selectTab(host, label);
    }

    expect(mounts.log).toEqual(mountedOnce);
  });

  it('loads the server settings once for the page, not once per tab visit', async () => {
    const { host } = await mountSettings();
    expect(form.loads).toBe(1);

    await selectTab(host, 'System');
    await selectTab(host, 'Shelves');

    expect(form.loads).toBe(1);
  });

  it('groups the panels into five tabs and shows only the active one', async () => {
    const { host } = await mountSettings();

    const tabLabels = [...host.querySelectorAll('[role="tab"]')].map((tab) => tab.textContent?.trim());
    expect(tabLabels).toEqual(['Shelves', 'General', 'Reading', 'Import & covers', 'System']);
    expect(panelsByTab(host).visible).toEqual([['shelves']]);

    await selectTab(host, 'General');
    expect(panelsByTab(host).visible).toEqual([['language', 'theme', 'nsfw']]);

    await selectTab(host, 'Reading');
    expect(panelsByTab(host).visible).toEqual([['reader-launch', 'read-history']]);

    await selectTab(host, 'Import & covers');
    expect(panelsByTab(host).visible).toEqual([['import', 'cover']]);

    await selectTab(host, 'System');
    const { visible, hidden } = panelsByTab(host);
    expect(visible).toEqual([['logs', 'about']]);
    expect(hidden).toHaveLength(4);
  });

  it('shows the browser storage panel under System only in the wasm demo', async () => {
    await mountSettings();
    expect(mounts.log).not.toContain('demo-storage');
    active?.app.unmount();
    active?.host.remove();

    window.plainshelfDemo = {
      persistEvent: 'plainshelf-demo-persist',
      hasStorage: () => Promise.resolve(true),
      canPersist: () => true,
      persisted: () => Promise.resolve(false),
      persist: () => Promise.resolve(true)
    };
    const demo = await mountSettings();
    await selectTab(demo.host, 'System');
    expect(panelsByTab(demo.host).visible).toEqual([['logs', 'demo-storage', 'about']]);
  });
});
