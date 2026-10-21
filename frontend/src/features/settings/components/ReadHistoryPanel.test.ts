// @vitest-environment jsdom
import { nextTick } from 'vue';
import { beforeEach, describe, expect, it } from 'vitest';

import ReadHistoryPanel from './ReadHistoryPanel.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

function mountPanel(value: number, disabled = false) {
  const changes: number[] = [];
  const { host } = mount(ReadHistoryPanel, {
    props: { value, disabled, onChange: (next: number) => changes.push(next) }
  });
  return { host, changes };
}

function field(host: HTMLElement): HTMLInputElement {
  const input = host.querySelector<HTMLInputElement>('input[role="spinbutton"]');
  if (!input) throw new Error('the read-history limit field is missing');
  return input;
}

// Reka UI 2.10.5 NumberField steppers handle pointer press/hold and match pointerup by pointerId.
const POINTER = { bubbles: true, button: 0, pointerId: 1, pointerType: 'mouse' };
function press(host: HTMLElement, direction: 'increase' | 'decrease'): void {
  const buttons = host.querySelectorAll<HTMLButtonElement>('.number-field-step');
  const button = direction === 'decrease' ? buttons[0] : buttons[1];
  button.dispatchEvent(new PointerEvent('pointerdown', POINTER));
  window.dispatchEvent(new PointerEvent('pointerup', POINTER));
}

beforeEach(() => {
  setLocale('en');
});

describe('ReadHistoryPanel', () => {
  it('shows the stored limit on a labelled spinbutton', () => {
    const { host } = mountPanel(20);

    const input = field(host);
    expect(input.value).toBe('20');
    expect(input.getAttribute('aria-valuemin')).toBe('0');
    const label = host.querySelector<HTMLLabelElement>('label');
    expect(label?.htmlFor).toBe(input.id);
  });

  it('reports a stepped limit and stops at zero', async () => {
    const { host, changes } = mountPanel(1);
    await nextTick();

    press(host, 'decrease');
    await nextTick();
    expect(changes).toEqual([0]);

    const { host: floorHost, changes: floorChanges } = mountPanel(0);
    await nextTick();
    press(floorHost, 'decrease');
    await nextTick();
    expect(floorChanges).toEqual([]);

  });

  it('restores the stored limit when the box is emptied, saving nothing', async () => {
    const { host, changes } = mountPanel(20);
    await nextTick();

    const input = field(host);
    input.value = '';
    input.dispatchEvent(new Event('input'));
    input.dispatchEvent(new Event('blur'));
    await nextTick();
    await nextTick();

    expect(changes).toEqual([]);
    expect(input.value).toBe('20');

    press(host, 'increase');
    await nextTick();
    expect(changes).toEqual([21]);
  });

  it('rounds a fractional limit to a whole number of entries', async () => {
    const { host, changes } = mountPanel(20);

    const input = field(host);
    input.value = '12.4';
    input.dispatchEvent(new Event('blur'));
    await nextTick();

    expect(changes).toEqual([12]);
  });

  it('disables the whole field while a save is in flight', () => {
    const { host } = mountPanel(20, true);

    for (const element of [field(host), ...host.querySelectorAll('.number-field-step')]) {
      expect(element.hasAttribute('disabled')).toBe(true);
      expect(element.hasAttribute('data-disabled')).toBe(true);
    }
    expect(host.querySelector('.number-field')?.hasAttribute('data-disabled')).toBe(true);

  });
});
