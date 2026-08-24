// @vitest-environment jsdom
import { createApp, defineComponent, h, reactive, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

// The reactive route the component reads. Tests flip `name` to move between the
// reader route and everything else; `fullPath` exists only so App.vue's
// renderError watch has something to watch.
const route = reactive<{ name: string | undefined; fullPath: string }>({
  name: 'library',
  fullPath: '/'
});

const runtime = vi.hoisted(() => ({ wails: false }));

vi.mock('./providers', () => ({
  isWailsRuntime: () => runtime.wails
}));

vi.mock('@/api/client', () => ({
  isMockApiMode: () => false
}));

vi.mock('./i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}));

vi.mock('@/components/ToastHost.vue', () => ({
  default: defineComponent({ setup: () => () => h('div') })
}));

vi.mock('vue-router', () => ({
  useRoute: () => route,
  RouterView: defineComponent({ setup: () => () => h('div', { class: 'router-view' }) })
}));

import AppComponent from './App.vue';

let mounted: { app: App; host: HTMLElement } | null = null;

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(AppComponent) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
}

function controls(host: HTMLElement): HTMLElement | null {
  return host.querySelector('.desktop-history-controls');
}

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  document.body.innerHTML = '';
  runtime.wails = false;
  route.name = 'library';
  route.fullPath = '/';
});

describe('App desktop history controls', () => {
  it('hides the controls on the reader route under the Wails runtime', () => {
    runtime.wails = true;
    route.name = 'reader';
    const host = mount();
    expect(controls(host)).toBeNull();
  });

  it('shows the controls on non-reader routes under the Wails runtime', () => {
    runtime.wails = true;
    route.name = 'book-detail';
    const host = mount();
    expect(controls(host)).not.toBeNull();
  });

  it('never renders the controls outside the Wails runtime', () => {
    runtime.wails = false;
    route.name = 'library';
    const host = mount();
    expect(controls(host)).toBeNull();
  });

  it('reveals the controls immediately when leaving the reader route', async () => {
    runtime.wails = true;
    route.name = 'reader';
    const host = mount();
    expect(controls(host)).toBeNull();

    route.name = 'book-detail';
    await flush();
    expect(controls(host)).not.toBeNull();
  });
});
