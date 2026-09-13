// @vitest-environment jsdom
import { nextTick } from 'vue';
import { beforeEach, describe, expect, it } from 'vitest';

import ReadHistoryPanel from './ReadHistoryPanel.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

// Captured emits stay here rather than in the shared helper: what a component
// reports is its own, and only this file knows what to do with it.
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

// Reka's steppers act on pointerdown (so a press-and-hold repeats), not click.
function press(host: HTMLElement, direction: 'increase' | 'decrease'): void {
  const buttons = host.querySelectorAll<HTMLButtonElement>('.number-field-step');
  const button = direction === 'decrease' ? buttons[0] : buttons[1];
  button.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 }));
  window.dispatchEvent(new MouseEvent('pointerup', { bubbles: true, button: 0 }));
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
    // The label is a real `<label for>`: reka's NumberFieldInput is an input.
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
    // A negative limit is not a state the server accepts, so the stepper is
    // disabled at the floor rather than reporting one.
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

    // Nothing to save, and the box must not sit blank over a setting that is
    // still in force.
    expect(changes).toEqual([]);
    expect(input.value).toBe('20');

    // The steppers read the box's text, so a box left blank would step from the
    // minimum instead of the stored value.
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

    // Reka pairs the native attribute with `data-disabled` on the input, both
    // steppers and the surrounding box; numeric-controls.css styles the
    // disabled state off the data attribute, which is the only one the box
    // itself can carry.
    for (const element of [field(host), ...host.querySelectorAll('.number-field-step')]) {
      expect(element.hasAttribute('disabled')).toBe(true);
      expect(element.hasAttribute('data-disabled')).toBe(true);
    }
    expect(host.querySelector('.number-field')?.hasAttribute('data-disabled')).toBe(true);

  });
});
