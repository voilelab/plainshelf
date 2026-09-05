// @vitest-environment jsdom
import { createApp, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NsfwPanel from './NsfwPanel.vue';
import { setLocale } from '@/i18n';

// Reka UI is used for real, as in CoverPanel.test.ts: the switch's own DOM
// contract is the point, and a stub would invent it rather than verify it.
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
  it('renders the setting as a switch that reflects the current value', () => {
    const { host, app } = mount({ value: true, disabled: false });
    mounted = app;

    const control = host.querySelector('[role="switch"]');
    expect(control).not.toBeNull();
    expect(control?.getAttribute('aria-checked')).toBe('true');
  });

  it('emits the next value as a boolean rather than a DOM event', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: false, onChange });
    mounted = app;

    host.querySelector<HTMLElement>('[role="switch"]')?.click();

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('names the switch from the title alone, not the whole row', () => {
    const { host, app } = mount({ value: false, disabled: false });
    mounted = app;

    const control = host.querySelector('[role="switch"]');
    const title = host.querySelector('.setting-label');

    expect(host.querySelector('label.setting-item')?.getAttribute('for')).toBe(control?.id);
    expect(control?.getAttribute('aria-labelledby')).toBe(title?.id);
  });

  it('says where the marks themselves come from, which this switch does not control', () => {
    const { host, app } = mount({ value: false, disabled: false });
    mounted = app;

    // The note sits outside the switch's row, so it is not part of the
    // control's accessible description and does not become a toggle target.
    const note = host.querySelector('.settings-note');
    expect(note?.textContent).toContain('shelf.json');
    expect(host.querySelector('label.setting-item')?.contains(note!)).toBe(false);
  });

  it('does not emit while the panel is saving', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: true, onChange });
    mounted = app;

    const control = host.querySelector<HTMLButtonElement>('[role="switch"]');
    expect(control?.disabled).toBe(true);
    control?.click();

    expect(onChange).not.toHaveBeenCalled();
  });
});
