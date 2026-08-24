// @vitest-environment jsdom
import { createApp, defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The gate under test is `writesEnabled`. Hold the ref in a hoisted box so each
// case can flip it before mounting; the real composable asks the active
// provider and server mode, which this component does not need to exercise.
const state = vi.hoisted(() => ({ writesEnabled: null as { value: boolean } | null }));

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref } = await import('vue');
  state.writesEnabled = ref(true);
  return { useWriteAccess: () => ({ writesEnabled: state.writesEnabled }) };
});

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
});
