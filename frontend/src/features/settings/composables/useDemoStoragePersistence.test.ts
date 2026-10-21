// @vitest-environment jsdom
import { createApp, defineComponent, nextTick } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { isWasmDemo, useDemoStoragePersistence } from './useDemoStoragePersistence';

async function flush(): Promise<void> {
  await nextTick();
  await new Promise((resolve) => setTimeout(resolve));
}

function fakeDemo(opts: { storage?: boolean; canPersist?: boolean; persisted?: boolean; grants?: boolean }) {
  let persisted = opts.persisted ?? false;
  const demo = {
    persistEvent: 'plainshelf-demo-persist',
    hasStorage: () => Promise.resolve(opts.storage ?? true),
    canPersist: () => opts.canPersist ?? true,
    persisted: () => Promise.resolve(persisted),
    persist: vi.fn(async () => {
      persisted = opts.grants ?? false;
      return persisted;
    }),
    grantElsewhere() {
      persisted = true;
      window.dispatchEvent(new CustomEvent('plainshelf-demo-persist'));
    }
  };
  window.plainshelfDemo = demo;
  return demo;
}

function mount() {
  let api: ReturnType<typeof useDemoStoragePersistence> | null = null;
  const app = createApp(
    defineComponent({
      setup() {
        api = useDemoStoragePersistence();
        return () => null;
      }
    })
  );
  app.mount(document.createElement('div'));
  return { api: api!, app };
}

afterEach(() => {
  delete window.plainshelfDemo;
});

describe('useDemoStoragePersistence', () => {
  it('detects the wasm demo by its global', () => {
    expect(isWasmDemo()).toBe(false);
    fakeDemo({});
    expect(isWasmDemo()).toBe(true);
  });

  it.each([
    [{ storage: false }, 'memory'],
    [{ canPersist: false }, 'unsupported'],
    [{ persisted: true }, 'persisted'],
    [{ persisted: true, canPersist: false }, 'persisted'],
    [{}, 'best-effort']
  ] as const)('reports %o as %s', async (opts, state) => {
    fakeDemo(opts);
    const { api, app } = mount();
    await flush();
    expect(api.state.value).toBe(state);
    app.unmount();
  });

  it('becomes persisted when the browser agrees', async () => {
    const demo = fakeDemo({ grants: true });
    const { api, app } = mount();
    await flush();
    await api.request();
    expect(demo.persist).toHaveBeenCalledOnce();
    expect(api.state.value).toBe('persisted');
    expect(api.declined.value).toBe(false);
    app.unmount();
  });

  it('stays best-effort and reports a decline', async () => {
    fakeDemo({ grants: false });
    const { api, app } = mount();
    await flush();
    await api.request();
    expect(api.state.value).toBe('best-effort');
    expect(api.declined.value).toBe(true);
    app.unmount();
  });

  it('follows a request made from the Demo bar', async () => {
    const demo = fakeDemo({});
    const { api, app } = mount();
    await flush();
    demo.grantElsewhere();
    await flush();
    expect(api.state.value).toBe('persisted');
    app.unmount();
  });
});
