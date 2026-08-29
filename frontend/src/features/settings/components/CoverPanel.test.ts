// @vitest-environment jsdom
import { createApp, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import CoverPanel from './CoverPanel.vue';
import { setLocale } from '@/i18n';

// Reka UI is used for real here: the point of these tests is the switch's own
// DOM contract (role, aria-checked, the disabled button), which a stub would
// invent rather than verify.
function mount(props: { value: boolean; disabled: boolean; onChange?: (value: boolean) => void }) {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const app = createApp(CoverPanel, props);
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

describe('CoverPanel', () => {
  it('renders the setting as a switch that reflects the current value', () => {
    const { host, app } = mount({ value: true, disabled: false });
    mounted = app;

    const control = host.querySelector('[role="switch"]');
    expect(control).not.toBeNull();
    expect(control?.getAttribute('aria-checked')).toBe('true');
    // The old control was an <input type="checkbox">; nothing should render one.
    expect(host.querySelector('input[type="checkbox"]')).toBeNull();
  });

  it('emits the next value as a boolean rather than a DOM event', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: false, onChange });
    mounted = app;

    host.querySelector<HTMLElement>('[role="switch"]')?.click();

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('names the switch from the row label, which is no longer wrapping it', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: false, onChange });
    mounted = app;

    const control = host.querySelector('[role="switch"]');
    const label = host.querySelector('label.setting-label');
    const description = host.querySelector('p.setting-description');

    // The switch is a <button>, so the row cannot be one big <label> any more.
    // These three links are what keeps it labelled and clickable instead.
    expect(label?.getAttribute('for')).toBe(control?.id);
    expect(control?.getAttribute('aria-labelledby')).toBe(label?.id);
    expect(control?.getAttribute('aria-describedby')).toBe(description?.id);
    expect(label?.closest('[role="switch"]')).toBeNull();

    // Clicking the text used to toggle the setting because the whole row was a
    // <label>. It still does, now through the for/id pair.
    (label as HTMLLabelElement | null)?.click();
    expect(onChange).toHaveBeenCalledWith(true);
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
