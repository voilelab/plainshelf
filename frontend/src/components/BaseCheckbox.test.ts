// @vitest-environment jsdom
import { createApp, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

import BaseCheckbox from './BaseCheckbox.vue';

// Reka UI is used for real: the point of these tests is the checkbox's own DOM
// contract — the role, aria-checked and the disabled button — which a stub
// would invent rather than verify.
function mount(props: {
  modelValue: boolean;
  disabled?: boolean;
  'onUpdate:modelValue'?: (value: boolean) => void;
}) {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const app = createApp(BaseCheckbox, props);
  app.mount(host);
  return { host, app };
}

let mounted: App | null = null;

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

describe('BaseCheckbox', () => {
  it('renders a checkbox that reflects the current value', () => {
    const { host, app } = mount({ modelValue: true });
    mounted = app;

    const control = host.querySelector('[role="checkbox"]');
    expect(control).not.toBeNull();
    expect(control?.getAttribute('aria-checked')).toBe('true');
    // The old control was an <input type="checkbox">; nothing renders one now.
    expect(host.querySelector('input[type="checkbox"]')).toBeNull();
  });

  it('emits the next value as a boolean', () => {
    const onUpdate = vi.fn();
    const { host, app } = mount({ modelValue: false, 'onUpdate:modelValue': onUpdate });
    mounted = app;

    host.querySelector<HTMLElement>('[role="checkbox"]')?.click();

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenCalledWith(true);
  });

  // Reka's model value also carries 'indeterminate'. Nothing here uses it, and
  // a caller typed to boolean must never be handed the string.
  it('never emits anything but a boolean', () => {
    const onUpdate = vi.fn();
    const { host, app } = mount({ modelValue: true, 'onUpdate:modelValue': onUpdate });
    mounted = app;

    host.querySelector<HTMLElement>('[role="checkbox"]')?.click();

    expect(onUpdate).toHaveBeenCalledWith(false);
    expect(typeof onUpdate.mock.calls[0][0]).toBe('boolean');
  });

  it('shows the tick only while checked', () => {
    const { host, app } = mount({ modelValue: false });
    mounted = app;
    expect(host.querySelector('svg')).toBeNull();
    app.unmount();

    const checked = mount({ modelValue: true });
    mounted = checked.app;
    expect(checked.host.querySelector('svg')).not.toBeNull();
  });

  it('does not emit while disabled', () => {
    const onUpdate = vi.fn();
    const { host, app } = mount({
      modelValue: false,
      disabled: true,
      'onUpdate:modelValue': onUpdate
    });
    mounted = app;

    const control = host.querySelector<HTMLButtonElement>('[role="checkbox"]');
    expect(control?.disabled).toBe(true);
    control?.click();

    expect(onUpdate).not.toHaveBeenCalled();
  });
});
