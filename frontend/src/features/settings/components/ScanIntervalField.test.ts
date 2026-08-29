// @vitest-environment jsdom
import { createApp, defineComponent, h, ref } from 'vue';
import { beforeEach, describe, expect, it } from 'vitest';

import ScanIntervalField from './ScanIntervalField.vue';
import { setLocale } from '@/i18n';

// A tiny host so the field is exercised through v-model, which is how both
// shelf modals use it: the parent holds the Go duration string.
function mount(initial: string) {
  const value = ref(initial);
  const host = document.createElement('div');
  const app = createApp(
    defineComponent({
      setup: () => () => h(ScanIntervalField, { modelValue: value.value, 'onUpdate:modelValue': (v: string) => (value.value = v) })
    })
  );
  app.mount(host);
  return { host, app, value };
}

function select(host: HTMLElement, testid: string): HTMLSelectElement {
  const el = host.querySelector<HTMLSelectElement>(`[data-testid="${testid}"]`);
  if (!el) {
    throw new Error(`missing ${testid}`);
  }
  return el;
}

function setValue(el: HTMLSelectElement | HTMLInputElement, value: string): void {
  el.value = value;
  el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? 'change' : 'input'));
}

beforeEach(() => {
  setLocale('en');
});

describe('ScanIntervalField', () => {
  it('loads a blank interval as the shelf default', () => {
    const { host, value, app } = mount('');

    expect(select(host, 'scan-interval-mode').value).toBe('default');
    expect(host.querySelector('[data-testid="scan-interval-amount"]')).toBeNull();
    expect(value.value).toBe('');

    app.unmount();
  });

  it('normalizes a compound duration into a single unit', () => {
    const { host, value, app } = mount('1h30m');

    expect(select(host, 'scan-interval-mode').value).toBe('interval');
    expect(select(host, 'scan-interval-amount').value).toBe('90');
    expect(select(host, 'scan-interval-unit').value).toBe('m');
    // What the controls show is what a save would write.
    expect(value.value).toBe('90m');

    app.unmount();
  });

  it('shows 0s as its own mode instead of a zero interval', () => {
    const { host, app } = mount('0s');

    expect(select(host, 'scan-interval-mode').value).toBe('always');
    expect(host.querySelector('[data-testid="scan-interval-amount"]')).toBeNull();

    app.unmount();
  });

  it('emits a Go duration when the amount and unit change', async () => {
    const { host, value, app } = mount('10m');

    setValue(select(host, 'scan-interval-amount'), '30');
    await Promise.resolve();
    expect(value.value).toBe('30m');

    setValue(select(host, 'scan-interval-unit'), 'h');
    await Promise.resolve();
    expect(value.value).toBe('30h');

    app.unmount();
  });

  it('never emits an invalid duration while the amount box is empty', async () => {
    const { host, value, app } = mount('10m');

    const amount = select(host, 'scan-interval-amount');
    setValue(amount, '');
    await Promise.resolve();
    expect(value.value).toBe('1m');

    amount.dispatchEvent(new Event('blur'));
    await Promise.resolve();
    expect(amount.value).toBe('1');

    app.unmount();
  });

  it('switches between the three modes', async () => {
    const { host, value, app } = mount('');

    setValue(select(host, 'scan-interval-mode'), 'always');
    await Promise.resolve();
    expect(value.value).toBe('0s');

    setValue(select(host, 'scan-interval-mode'), 'interval');
    await Promise.resolve();
    expect(value.value).toBe('1m');

    setValue(select(host, 'scan-interval-mode'), 'default');
    await Promise.resolve();
    expect(value.value).toBe('');

    app.unmount();
  });

  it('replaces a duration it cannot represent and says so', async () => {
    const { host, value, app } = mount('1500ms');

    expect(value.value).toBe('2s');
    expect(select(host, 'scan-interval-amount').value).toBe('2');
    expect(host.querySelector('.scan-interval-adjusted')?.textContent).toContain('1500ms');

    // The note goes away once the user picks a value of their own.
    setValue(select(host, 'scan-interval-amount'), '5');
    await Promise.resolve();
    expect(value.value).toBe('5s');
    expect(host.querySelector('.scan-interval-adjusted')).toBeNull();

    app.unmount();
  });
});
