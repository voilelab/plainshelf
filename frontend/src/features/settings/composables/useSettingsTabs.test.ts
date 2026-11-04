// @vitest-environment jsdom
import { createApp, defineComponent, nextTick, ref } from 'vue';
import { createMemoryHistory, createRouter, type Router } from 'vue-router';
import { describe, expect, it } from 'vitest';

import { useSettingsTabs } from './useSettingsTabs';

async function flush(): Promise<void> {
  await nextTick();
  await Promise.resolve();
  await new Promise((resolve) => setTimeout(resolve));
}

async function setup(editable: boolean, initialPath = '/settings') {
  const router: Router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/settings', component: defineComponent({ render: () => null }) }]
  });
  router.push(initialPath);
  await router.isReady();

  let api: ReturnType<typeof useSettingsTabs> | null = null;
  const Host = defineComponent({
    setup() {
      api = useSettingsTabs(ref(editable));
      return () => null;
    }
  });
  const app = createApp(Host);
  app.use(router);
  app.mount(document.createElement('div'));

  return { api: api!, router, app };
}

describe('useSettingsTabs', () => {
  it('defaults to general when server settings are editable', async () => {
    const { api, app } = await setup(true);
    expect(api.activeSettingsTab.value).toBe('general');
    app.unmount();
  });

  it('defaults to shelves when server settings are not editable', async () => {
    const { api, app } = await setup(false);
    expect(api.activeSettingsTab.value).toBe('shelves');
    app.unmount();
  });

  it('honours a ?tab= deep link', async () => {
    const { api, app } = await setup(true, '/settings?tab=shelves');
    expect(api.activeSettingsTab.value).toBe('shelves');
    app.unmount();
  });

  it('honours ?tab=reading on a read-only server', async () => {
    const { api, app } = await setup(false, '/settings?tab=reading');
    expect(api.activeSettingsTab.value).toBe('reading');
    app.unmount();
  });

  it('ignores ?tab=import when server settings are not editable', async () => {
    const { api, app } = await setup(false, '/settings?tab=import');
    expect(api.activeSettingsTab.value).toBe('shelves');
    app.unmount();
  });

  it('ignores a tab id that no longer exists', async () => {
    const { api, app } = await setup(true, '/settings?tab=cover');
    expect(api.activeSettingsTab.value).toBe('general');
    app.unmount();
  });

  it('writes the selected tab back into the query', async () => {
    const { api, router, app } = await setup(true, '/settings?tab=shelves');

    api.activeSettingsTab.value = 'reading';
    await flush();

    expect(router.currentRoute.value.query.tab).toBe('reading');
    expect(api.activeSettingsTab.value).toBe('reading');
    app.unmount();
  });

  it('returns to shelves when the manage link is followed after a tab switch', async () => {
    const { api, router, app } = await setup(true, '/settings?tab=shelves');

    api.activeSettingsTab.value = 'import';
    await flush();
    expect(router.currentRoute.value.query.tab).toBe('import');

    await router.push('/settings?tab=shelves');
    await flush();
    expect(api.activeSettingsTab.value).toBe('shelves');
    app.unmount();
  });
});
