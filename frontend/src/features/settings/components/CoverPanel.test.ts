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

  it('names the switch from the title alone, not the whole row', () => {
    const { host, app } = mount({ value: false, disabled: false });
    mounted = app;

    const control = host.querySelector('[role="switch"]');
    const title = host.querySelector('.setting-label');
    const description = host.querySelector('p.setting-description');

    expect(host.querySelector('label.setting-item')?.getAttribute('for')).toBe(control?.id);
    // Without these the accessible name would be the row's whole text, the
    // description included.
    expect(control?.getAttribute('aria-labelledby')).toBe(title?.id);
    expect(control?.getAttribute('aria-describedby')).toBe(description?.id);
  });

  it('keeps the whole row a click target, description included', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: false, onChange });
    mounted = app;

    // The row was one big <label> around the checkbox, so anywhere in it
    // toggled the setting. A switch is a <button>, and pointing the same label
    // at it by id is what preserves that.
    host.querySelector<HTMLElement>('p.setting-description')?.click();
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(true);

    host.querySelector<HTMLElement>('.setting-label')?.click();
    expect(onChange).toHaveBeenCalledTimes(2);
  });

  it('fires once when the switch itself is clicked inside the row label', () => {
    const onChange = vi.fn();
    const { host, app } = mount({ value: false, disabled: false, onChange });
    mounted = app;

    // A label does nothing for clicks targeting interactive content inside it,
    // so the switch does not also receive the label's synthetic click.
    host.querySelector<HTMLElement>('[role="switch"]')?.click();

    expect(onChange).toHaveBeenCalledTimes(1);
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
