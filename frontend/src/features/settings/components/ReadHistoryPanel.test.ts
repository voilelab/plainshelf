// @vitest-environment jsdom
import { createApp, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import ReadHistoryPanel from './ReadHistoryPanel.vue';
import { setLocale } from '@/i18n';

function mount(value: number, disabled = false) {
  const host = document.createElement('div');
  const changes: number[] = [];
  const app = createApp(ReadHistoryPanel, {
    value,
    disabled,
    onChange: (next: number) => changes.push(next)
  });
  app.mount(host);
  return { host, app, changes };
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

afterEach(() => {
  document.body.innerHTML = '';
});

describe('ReadHistoryPanel', () => {
  it('shows the stored limit on a labelled spinbutton', () => {
    const { host, app } = mount(20);

    const input = field(host);
    expect(input.value).toBe('20');
    expect(input.getAttribute('aria-valuemin')).toBe('0');
    // The label is a real `<label for>`: reka's NumberFieldInput is an input.
    const label = host.querySelector<HTMLLabelElement>('label');
    expect(label?.htmlFor).toBe(input.id);

    app.unmount();
  });

  it('reports a stepped limit and stops at zero', async () => {
    const { host, app, changes } = mount(1);
    await nextTick();

    press(host, 'decrease');
    await nextTick();
    expect(changes).toEqual([0]);

    const { host: floorHost, app: floorApp, changes: floorChanges } = mount(0);
    await nextTick();
    press(floorHost, 'decrease');
    await nextTick();
    // A negative limit is not a state the server accepts, so the stepper is
    // disabled at the floor rather than reporting one.
    expect(floorChanges).toEqual([]);

    app.unmount();
    floorApp.unmount();
  });

  it('reports nothing for an emptied box instead of saving NaN', async () => {
    const { host, app, changes } = mount(20);

    const input = field(host);
    input.value = '';
    input.dispatchEvent(new Event('blur'));
    await nextTick();

    expect(changes).toEqual([]);

    app.unmount();
  });

  it('rounds a fractional limit to a whole number of entries', async () => {
    const { host, app, changes } = mount(20);

    const input = field(host);
    input.value = '12.4';
    input.dispatchEvent(new Event('blur'));
    await nextTick();

    expect(changes).toEqual([12]);

    app.unmount();
  });

  it('disables the whole field while a save is in flight', () => {
    const { host, app } = mount(20, true);

    expect(field(host).hasAttribute('disabled')).toBe(true);
    for (const button of host.querySelectorAll('.number-field-step')) {
      expect(button.hasAttribute('disabled')).toBe(true);
    }

    app.unmount();
  });
});
