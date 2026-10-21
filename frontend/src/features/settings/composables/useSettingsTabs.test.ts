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

async function setup(editable: boolean, initialPath = '/settings', wasmDemo = false) {
  const router: Router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/settings', component: defineComponent({ render: () => null }) }]
  });
  router.push(initialPath);
  await router.isReady();

  let api: ReturnType<typeof useSettingsTabs> | null = null;
  const Host = defineComponent({
    setup() {
      api = useSettingsTabs(ref(editable), wasmDemo);
      return () => null;
    }
  });
  const app = createApp(Host);
  app.use(router);
  app.mount(document.createElement('div'));

  return { api: api!, router, app };
}

describe('useSettingsTabs', () => {
  it('defaults to cover when server settings are editable', async () => {
    const { api, app } = await setup(true);
    expect(api.activeSettingsTab.value).toBe('cover');
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

  it('honours ?tab=language on a read-only server', async () => {
    const { api, app } = await setup(false, '/settings?tab=language');
    expect(api.activeSettingsTab.value).toBe('language');
    app.unmount();
  });

  it('honours ?tab=device-nsfw only on a client with no server settings', async () => {
    const readOnly = await setup(false, '/settings?tab=device-nsfw');
    expect(readOnly.api.activeSettingsTab.value).toBe('device-nsfw');
    readOnly.app.unmount();

    const editable = await setup(true, '/settings?tab=device-nsfw');
    expect(editable.api.activeSettingsTab.value).toBe('cover');
    editable.app.unmount();
  });

  it('honours ?tab=demo-storage only in the wasm demo', async () => {
    const demo = await setup(true, '/settings?tab=demo-storage', true);
    expect(demo.api.activeSettingsTab.value).toBe('demo-storage');
    demo.app.unmount();

    const server = await setup(true, '/settings?tab=demo-storage');
    expect(server.api.activeSettingsTab.value).toBe('cover');
    server.app.unmount();
  });

  it('ignores a tab the current shell does not show', async () => {
    const { api, app } = await setup(false, '/settings?tab=cover');
    expect(api.activeSettingsTab.value).toBe('shelves');
    app.unmount();
  });

  it('writes the selected tab back into the query', async () => {
    const { api, router, app } = await setup(true, '/settings?tab=shelves');

    api.activeSettingsTab.value = 'read-history';
    await flush();

    expect(router.currentRoute.value.query.tab).toBe('read-history');
    expect(api.activeSettingsTab.value).toBe('read-history');
    app.unmount();
  });

  it('returns to shelves when the manage link is followed after a tab switch', async () => {
    const { api, router, app } = await setup(true, '/settings?tab=shelves');

    api.activeSettingsTab.value = 'cover';
    await flush();
    expect(router.currentRoute.value.query.tab).toBe('cover');

    await router.push('/settings?tab=shelves');
    await flush();
    expect(api.activeSettingsTab.value).toBe('shelves');
    app.unmount();
  });
});
