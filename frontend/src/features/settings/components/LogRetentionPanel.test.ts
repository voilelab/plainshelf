// @vitest-environment jsdom
import { createApp, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import LogRetentionPanel from './LogRetentionPanel.vue';
import { setLocale } from '@/i18n';

function mount(value: number) {
  const host = document.createElement('div');
  const changes: number[] = [];
  const app = createApp(LogRetentionPanel, {
    value,
    disabled: false,
    onChange: (next: number) => changes.push(next)
  });
  app.mount(host);
  return { host, app, changes };
}

function field(host: HTMLElement): HTMLInputElement {
  const input = host.querySelector<HTMLInputElement>('input[role="spinbutton"]');
  if (!input) throw new Error('the retention field is missing');
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

describe('LogRetentionPanel', () => {
  // The panel deletes files, so what the current number does has to be legible
  // without the reader working it out from the unit.
  it('says in words that a window deletes older files', () => {
    const { host, app } = mount(30);

    expect(host.textContent).toContain('Log files older than 30 days are deleted.');

    app.unmount();
  });

  it('says that zero deletes nothing', () => {
    const { host, app } = mount(0);

    expect(host.textContent).toContain('No log file is deleted.');
    // The static description also says "are deleted when the log rotates", so
    // match the effect line's own wording rather than the phrase alone.
    expect(host.textContent).not.toContain('Log files older than');

    app.unmount();
  });

  it('bounds the field at what the server accepts', () => {
    const { host, app } = mount(30);

    // Reka renders a text input with the spinbutton role, so the bounds live on
    // the ARIA attributes rather than on `min`/`max`.
    const input = field(host);
    expect(input.getAttribute('aria-valuemin')).toBe('0');
    expect(input.getAttribute('aria-valuemax')).toBe('3650');
    expect(input.value).toBe('30');

    app.unmount();
  });

  it('reports a stepped value and clamps it at the server maximum', async () => {
    const { host, app, changes } = mount(3649);
    await nextTick();

    press(host, 'increase');
    await nextTick();
    expect(changes).toEqual([3650]);

    // At the ceiling the stepper is disabled rather than reporting a value the
    // server would reject.
    const { host: cappedHost, app: cappedApp, changes: cappedChanges } = mount(3650);
    await nextTick();
    press(cappedHost, 'increase');
    await nextTick();
    expect(cappedChanges).toEqual([]);

    app.unmount();
    cappedApp.unmount();
  });

  it('reports nothing for an emptied box instead of saving NaN', async () => {
    const { host, app, changes } = mount(30);

    const input = field(host);
    input.value = '';
    input.dispatchEvent(new Event('blur'));
    await nextTick();

    expect(changes).toEqual([]);

    app.unmount();
  });

  it('reports a typed window when the box loses focus', async () => {
    const { host, app, changes } = mount(30);

    const input = field(host);
    input.value = '90';
    input.dispatchEvent(new Event('blur'));
    await nextTick();

    expect(changes).toEqual([90]);

    app.unmount();
  });
});
