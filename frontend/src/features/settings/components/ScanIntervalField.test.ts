// @vitest-environment jsdom
import { nextTick, ref } from 'vue';
import { beforeEach, describe, expect, it } from 'vitest';

import ScanIntervalField from './ScanIntervalField.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

function mountField(initial: string) {
  const value = ref(initial);
  const { host } = mount(ScanIntervalField, {
    props: () => ({
      modelValue: value.value,
      'onUpdate:modelValue': (next: string) => (value.value = next)
    })
  });
  return { host, value };
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
    const { host, value } = mountField('');

    expect(select(host, 'scan-interval-mode').value).toBe('default');
    expect(host.querySelector('[data-testid="scan-interval-amount"]')).toBeNull();
    expect(value.value).toBe('');
  });

  it('normalizes a compound duration into a single unit', () => {
    const { host, value } = mountField('1h30m');

    expect(select(host, 'scan-interval-mode').value).toBe('interval');
    expect(amountBox(host).value).toBe('90');
    expect(select(host, 'scan-interval-unit').value).toBe('m');
    expect(value.value).toBe('90m');
  });

  it('shows 0s as its own mode instead of a zero interval', () => {
    const { host } = mountField('0s');

    expect(select(host, 'scan-interval-mode').value).toBe('always');
    expect(host.querySelector('[data-testid="scan-interval-amount"]')).toBeNull();
  });

  it('emits a Go duration when the amount and unit change', async () => {
    const { host, value } = mountField('10m');

    commit(amountBox(host), '30');
    await nextTick();
    expect(value.value).toBe('30m');

    setValue(select(host, 'scan-interval-unit'), 'h');
    await nextTick();
    expect(value.value).toBe('30h');
  });

  it('never emits an invalid duration while the amount box is empty', async () => {
    const { host, value } = mountField('10m');

    const amount = amountBox(host);
    commit(amount, '');
    await nextTick();
    await nextTick();

    expect(value.value).toBe('10m');
    expect(amount.value).toBe('10');
  });

  it('caps an amount past what Go can parse', async () => {
    const { host, value } = mountField('10h');

    const amount = amountBox(host);
    expect(amount.getAttribute('aria-valuemax')).toBe('2562047');

    commit(amount, '2562048');
    await nextTick();
    expect(value.value).toBe('2562047h');

    await nextTick();
    expect(amount.value).toBe('2562047');
  });

  it('re-caps the amount when a larger unit is chosen', async () => {
    const { host, value } = mountField('1s');

    const amount = amountBox(host);
    commit(amount, '9223372036');
    await nextTick();
    expect(value.value).toBe('9223372036s');

    setValue(select(host, 'scan-interval-unit'), 'h');
    await nextTick();
    await nextTick();

    expect(value.value).toBe('2562047h');
    expect(amount.value).toBe('2562047');
  });

  it('switches between the three modes', async () => {
    const { host, value } = mountField('');

    setValue(select(host, 'scan-interval-mode'), 'always');
    await Promise.resolve();
    expect(value.value).toBe('0s');

    setValue(select(host, 'scan-interval-mode'), 'interval');
    await Promise.resolve();
    expect(value.value).toBe('1m');

    setValue(select(host, 'scan-interval-mode'), 'default');
    await Promise.resolve();
    expect(value.value).toBe('');
  });

  it('replaces a duration it cannot represent and says so', async () => {
    const { host, value } = mountField('1500ms');

    expect(value.value).toBe('2s');
    expect(amountBox(host).value).toBe('2');
    expect(host.querySelector('.scan-interval-adjusted')?.textContent).toContain('1500ms');

    commit(amountBox(host), '5');
    await nextTick();
    expect(value.value).toBe('5s');
    expect(host.querySelector('.scan-interval-adjusted')).toBeNull();
  });
});
