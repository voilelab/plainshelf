// @vitest-environment jsdom
import { createApp, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NsfwPanel from './NsfwPanel.vue';
import { setLocale } from '@/i18n';

// Only what is this panel's own. The switch's DOM contract — role, aria-checked,
// the row label pointing at it by id, the disabled button — belongs to
// BaseSwitch and the shared row markup, and CoverPanel.test.ts already pins it.
function mount(props: { value: boolean; disabled: boolean; onChange?: (value: boolean) => void }) {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const app = createApp(NsfwPanel, props);
  app.mount(host);
  return { host, app };
}

let mounted: App | null = null;

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

describe('NsfwPanel', () => {
  it('reflects the current value and emits the next one as a boolean', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: false, onChange });
    mounted = app;

    const control = host.querySelector<HTMLElement>('[role="switch"]');
    expect(control?.getAttribute('aria-checked')).toBe('false');
    control?.click();

    expect(onChange).toHaveBeenCalledExactlyOnceWith(true);
  });

  it('says where the marks come from, outside the row the switch controls', () => {
    const { host, app } = mount({ value: false, disabled: false });
    mounted = app;

    // Outside the label, so it neither joins the switch's accessible
    // description nor becomes another way to toggle the setting.
    const note = host.querySelector('.settings-note');
    expect(note?.textContent).toContain('shelf.json');
    expect(host.querySelector('label.setting-item')?.contains(note!)).toBe(false);
  });
});
