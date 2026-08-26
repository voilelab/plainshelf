// @vitest-environment jsdom
import { createApp, defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The gate under test is `writesEnabled`. Hold the ref in a hoisted box so each
// case can flip it before mounting; the real composable asks the active
// provider and server mode, which this component does not need to exercise.
const state = vi.hoisted(() => ({
  writesEnabled: null as { value: boolean } | null,
  selectedShelfID: null as { value: string } | null,
  provider: {} as Record<string, unknown>
}));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref } = await import('vue');
  state.writesEnabled = ref(true);
  return { useWriteAccess: () => ({ writesEnabled: state.writesEnabled }) };
});

vi.mock('@/composables/useShelvesStore', async () => {
  const { ref } = await import('vue');
  state.selectedShelfID = ref('');
  return { useShelvesStore: () => ({ selectedShelfID: state.selectedShelfID }) };
});

vi.mock('@/providers', () => ({ getBookshelfProvider: () => state.provider }));

import EmptyShelf from './EmptyShelf.vue';
import { setLocale } from '@/i18n';

// RouterLink is registered globally by the app; stub it so the import CTA is
// present and inspectable without standing up a router.
const RouterLinkStub = defineComponent({
  props: { to: { type: [String, Object], default: '' } },
  setup(props, { slots }) {
    return () =>
      h('a', { class: 'router-link-stub', 'data-to': JSON.stringify(props.to) }, slots.default?.());
  }
});

function mount() {
  const host = document.createElement('div');
  const app = createApp(EmptyShelf);
  app.component('RouterLink', RouterLinkStub);
  app.mount(host);
  return { host, app };
}

beforeEach(() => {
  setLocale('en');
  state.writesEnabled!.value = true;
  state.selectedShelfID!.value = '';
  state.provider = {};
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('EmptyShelf', () => {
  it('offers the import action when the shelf is writable', () => {
    state.writesEnabled!.value = true;
    const { host, app } = mount();

    const importLink = host.querySelector('.empty-shelf-import');
    expect(importLink).not.toBeNull();
    expect(importLink?.getAttribute('data-to')).toContain('import');
    expect(host.querySelector('.empty-shelf-description')?.textContent).toContain(
      'import them here'
    );

    app.unmount();
  });

  it('hides the import action and reads as read-only when writes are unavailable', () => {
    // A read-only mobile/pCloud client or a read-only server: the import query
    // would be stripped or the modal suppressed, so the CTA must not appear.
    state.writesEnabled!.value = false;
    const { host, app } = mount();

    expect(host.querySelector('.empty-shelf-import')).toBeNull();
    // The docs link stays; only the write action is gated.
    expect(host.querySelector('.empty-shelf-docs')).not.toBeNull();
    expect(host.querySelector('.empty-shelf-description')?.textContent).toContain(
      'no books yet'
    );

    app.unmount();
  });

  it('shows the active shelf folder path on the desktop and reveals it on click', async () => {
    const openDesktopShelfFolder = vi.fn(() => Promise.resolve());
    state.selectedShelfID!.value = 'my-books';
    state.provider = {
      getDesktopShelfDetails: vi.fn(() =>
        Promise.resolve({ id: 'my-books', name: 'My Books', path: '/home/reader/shelf', scan_interval: '' })
      ),
      openDesktopShelfFolder
    };
    const { host, app } = mount();

    await vi.waitFor(() =>
      expect(host.querySelector('.empty-shelf-path-value')?.textContent).toContain('/home/reader/shelf')
    );
    expect(host.querySelector('.empty-shelf-path-label')?.textContent).toContain('Shelf folder');

    host.querySelector<HTMLButtonElement>('.empty-shelf-path-value')?.click();
    expect(openDesktopShelfFolder).toHaveBeenCalledWith('my-books');

    app.unmount();
  });

  it('omits the folder path off the desktop', async () => {
    // The web/server provider exposes no getDesktopShelfDetails, so there is no
    // local path to show.
    state.selectedShelfID!.value = 'my-books';
    state.provider = {};
    const { host, app } = mount();

    await Promise.resolve();
    expect(host.querySelector('.empty-shelf-path')).toBeNull();

    app.unmount();
  });
});
