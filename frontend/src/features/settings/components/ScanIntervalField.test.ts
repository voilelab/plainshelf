// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, ref } from 'vue';
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

function amountBox(host: HTMLElement): HTMLInputElement {
  const el = host.querySelector<HTMLInputElement>('[data-testid="scan-interval-amount"]');
  if (!el) {
    throw new Error('missing scan-interval-amount');
  }
  return el;
}

function setValue(el: HTMLSelectElement, value: string): void {
  el.value = value;
  el.dispatchEvent(new Event('change'));
}

// Reka's NumberField commits on blur, Enter and the steppers rather than on
// every keystroke, so typing alone reports nothing.
function commit(el: HTMLInputElement, value: string): void {
  el.value = value;
  el.dispatchEvent(new Event('input'));
  el.dispatchEvent(new Event('blur'));
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
    expect(amountBox(host).value).toBe('90');
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

    commit(amountBox(host), '30');
    await nextTick();
    expect(value.value).toBe('30m');

    setValue(select(host, 'scan-interval-unit'), 'h');
    await nextTick();
    expect(value.value).toBe('30h');

    app.unmount();
  });

  it('never emits an invalid duration while the amount box is empty', async () => {
    const { host, value, app } = mount('10m');

    const amount = amountBox(host);
    commit(amount, '');
    await nextTick();
    await nextTick();

    // There is nothing to build a duration from, so the interval in force
    // stands and the box is redrawn with it rather than left blank over it.
    expect(value.value).toBe('10m');
    expect(amount.value).toBe('10');

    app.unmount();
  });

  it('caps an amount past what Go can parse', async () => {
    const { host, value, app } = mount('10h');

    const amount = amountBox(host);
    expect(amount.getAttribute('aria-valuemax')).toBe('2562047');

    commit(amount, '2562048');
    await nextTick();
    // The clamped value is one the backend accepts, so the raw
    // `invalid duration` error still cannot reach the user.
    expect(value.value).toBe('2562047h');

    await nextTick();
    expect(amount.value).toBe('2562047');

    app.unmount();
  });

  it('re-caps the amount when a larger unit is chosen', async () => {
    const { host, value, app } = mount('1s');

    const amount = amountBox(host);
    commit(amount, '9223372036');
    await nextTick();
    expect(value.value).toBe('9223372036s');

    setValue(select(host, 'scan-interval-unit'), 'h');
    await nextTick();
    await nextTick();

    // An hour holds 3600 times what a second does, so the ceiling drops with
    // the unit; carried over unchanged this would be a duration
    // `time.ParseDuration` rejects outright.
    expect(value.value).toBe('2562047h');
    expect(amount.value).toBe('2562047');

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
    expect(amountBox(host).value).toBe('2');
    expect(host.querySelector('.scan-interval-adjusted')?.textContent).toContain('1500ms');

    // The note goes away once the user picks a value of their own.
    commit(amountBox(host), '5');
    await nextTick();
    expect(value.value).toBe('5s');
    expect(host.querySelector('.scan-interval-adjusted')).toBeNull();

    app.unmount();
  });
});
