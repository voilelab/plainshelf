// @vitest-environment jsdom
import { nextTick } from 'vue';
import { beforeEach, describe, expect, it } from 'vitest';

import LogRetentionPanel from './LogRetentionPanel.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

function mountPanel(value: number) {
  const changes: number[] = [];
  const { host } = mount(LogRetentionPanel, {
    props: { value, disabled: false, onChange: (next: number) => changes.push(next) }
  });
  return { host, changes };
}

function field(host: HTMLElement): HTMLInputElement {
  const input = host.querySelector<HTMLInputElement>('input[role="spinbutton"]');
  if (!input) throw new Error('the retention field is missing');
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

describe('LogRetentionPanel', () => {
  it('says in words that a window deletes older files', () => {
    const { host } = mountPanel(30);

    expect(host.textContent).toContain('Log files older than 30 days are deleted.');
  });

  it('says that zero deletes nothing', () => {
    const { host } = mountPanel(0);

    expect(host.textContent).toContain('No log file is deleted.');
    expect(host.textContent).not.toContain('Log files older than');
  });

  it('bounds the field at what the server accepts', () => {
    const { host } = mountPanel(30);

    const input = field(host);
    expect(input.getAttribute('aria-valuemin')).toBe('0');
    expect(input.getAttribute('aria-valuemax')).toBe('3650');
    expect(input.value).toBe('30');
  });

  it('reports a stepped value and clamps it at the server maximum', async () => {
    const { host, changes } = mountPanel(3649);
    await nextTick();

    press(host, 'increase');
    await nextTick();
    expect(changes).toEqual([3650]);

    const { host: cappedHost, changes: cappedChanges } = mountPanel(3650);
    await nextTick();
    press(cappedHost, 'increase');
    await nextTick();
    expect(cappedChanges).toEqual([]);

  });

  it('restores the stored retention window when the box is emptied, saving nothing', async () => {
    const { host, changes } = mountPanel(30);
    await nextTick();

    const input = field(host);
    input.value = '';
    input.dispatchEvent(new Event('input'));
    input.dispatchEvent(new Event('blur'));
    await nextTick();
    await nextTick();

    expect(changes).toEqual([]);
    expect(input.value).toBe('30');

    press(host, 'increase');
    await nextTick();
    expect(changes).toEqual([31]);
  });

  it('reports a typed window when the box loses focus', async () => {
    const { host, changes } = mountPanel(30);

    const input = field(host);
    input.value = '90';
    input.dispatchEvent(new Event('blur'));
    await nextTick();

    expect(changes).toEqual([90]);

  });
});
