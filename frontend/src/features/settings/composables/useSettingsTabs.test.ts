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

// Mounts the composable inside a real router context and returns its handle
// plus the router, so a test can drive navigation and read the active tab.
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
    // Language moved out of the top bar into Settings; it must stay reachable
    // on the read-only mobile shell, so it lives outside EDITABLE_TABS.
    const { api, app } = await setup(false, '/settings?tab=language');
    expect(api.activeSettingsTab.value).toBe('language');
    app.unmount();
  });

  // The device's own adult-content answer is the mirror image of `cover`: it is
  // shown only where the server's `nsfw` tab is not, so one question never has
  // two switches in front of a user whose server already decides it.
  it('honours ?tab=device-nsfw only on a client with no server settings', async () => {
    const readOnly = await setup(false, '/settings?tab=device-nsfw');
    expect(readOnly.api.activeSettingsTab.value).toBe('device-nsfw');
    readOnly.app.unmount();

    const editable = await setup(true, '/settings?tab=device-nsfw');
    expect(editable.api.activeSettingsTab.value).toBe('cover');
    editable.app.unmount();
  });

  it('ignores a tab the current shell does not show', async () => {
    // cover is editable-only; on a read-only server it falls back to default.
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
    // The regression the fix guards: deep-link to Shelves, switch to Cover,
    // then follow /settings?tab=shelves again. Because switching to Cover wrote
    // ?tab=cover, the second navigation is a real query change and lands back
    // on Shelves — rather than re-targeting the already-current URL and being
    // ignored.
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
