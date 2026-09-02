// @vitest-environment jsdom
import { createApp, h, nextTick, reactive, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import CharCountFilterBar from './CharCountFilterBar.vue';
import { setLocale } from '@/i18n';
import { CHAR_COUNT_STEP, type CharCountRange } from '@/utils/charCountFilter';

interface Harness {
  app: App;
  host: HTMLElement;
  state: { range: CharCountRange };
  ranges: CharCountRange[];
}

const mounted: Harness[] = [];

// The bar is controlled: mirror each emit back into the prop so the fields show
// what a real parent would render on the next tick.
function mount(range: CharCountRange = {}): Harness {
  const host = document.createElement('div');
  document.body.append(host);

  const state = reactive({ range }) as { range: CharCountRange };
  const ranges: CharCountRange[] = [];

  const app = createApp({
    setup: () => () =>
      h(CharCountFilterBar, {
        range: state.range,
        unknownCount: 0,
        readOnly: false,
        refreshRunning: false,
        refreshLabel: 'Update statistics',
        refreshOutcome: '',
        refreshError: '',
        'onUpdate:range': (next: CharCountRange) => {
          ranges.push(next);
          state.range = next;
        }
      })
  });
  app.mount(host);

  const entry: Harness = { app, host, state, ranges };
  mounted.push(entry);
  return entry;
}

function bound(host: HTMLElement, which: 'min' | 'max'): HTMLInputElement {
  const inputs = host.querySelectorAll<HTMLInputElement>('input[role="spinbutton"]');
  const input = which === 'min' ? inputs[0] : inputs[1];
  if (!input) throw new Error(`the ${which} bound is missing`);
  return input;
}

// Reka's steppers act on pointerdown (so a press-and-hold repeats), not click.
function press(host: HTMLElement, which: 'min' | 'max', direction: 'increase' | 'decrease'): void {
  const buttons = host.querySelectorAll<HTMLButtonElement>('.number-field-step');
  const offset = which === 'min' ? 0 : 2;
  const button = buttons[offset + (direction === 'decrease' ? 0 : 1)];
  button.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 }));
  window.dispatchEvent(new MouseEvent('pointerup', { bubbles: true, button: 0 }));
}

function commit(input: HTMLInputElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('blur'));
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
  document.body.innerHTML = '';
});

describe('CharCountFilterBar bounds', () => {
  it('starts both bounds empty for an unlimited range', () => {
    const { host } = mount({});

    expect(bound(host, 'min').value).toBe('');
    expect(bound(host, 'max').value).toBe('');
  });

  it('commits a typed bound when the box loses focus', async () => {
    const { host, ranges } = mount({});

    commit(bound(host, 'max'), '5000');
    await nextTick();

    expect(ranges).toEqual([{ min: undefined, max: 5000 }]);
    // Rendered without a thousands separator, which is also what the URL and
    // the e2e suite read back.
    expect(bound(host, 'max').value).toBe('5000');
  });

  it('keeps a bound that is not a multiple of the stepper increment', async () => {
    const { host, ranges } = mount({});

    // 74 characters is a real book length; snapping it onto the 100-character
    // step grid would silently search for something else.
    commit(bound(host, 'max'), '74');
    await nextTick();

    expect(ranges).toEqual([{ min: undefined, max: 74 }]);
  });

  it('clears a bound when its box is emptied, without emitting NaN', async () => {
    const { host, ranges } = mount({ min: 100, max: 5000 });

    commit(bound(host, 'min'), '');
    await nextTick();

    expect(ranges).toEqual([{ min: undefined, max: 5000 }]);
  });

  it('stores reversed bounds in order', async () => {
    const { host, ranges } = mount({});

    commit(bound(host, 'min'), '5000');
    await nextTick();
    commit(bound(host, 'max'), '10');
    await nextTick();

    expect(ranges.at(-1)).toEqual({ min: 10, max: 5000 });
    expect(bound(host, 'min').value).toBe('10');
    expect(bound(host, 'max').value).toBe('5000');
  });

  it('steps a bound by CHAR_COUNT_STEP and stops at zero', async () => {
    const { host, ranges } = mount({ min: CHAR_COUNT_STEP });
    await nextTick();

    press(host, 'min', 'increase');
    await nextTick();
    expect(ranges.at(-1)).toEqual({ min: CHAR_COUNT_STEP * 2, max: undefined });

    press(host, 'min', 'decrease');
    await nextTick();
    expect(ranges.at(-1)).toEqual({ min: CHAR_COUNT_STEP, max: undefined });
  });

  it('does not step a bound below zero', async () => {
    const { host, ranges } = mount({ min: 0 });
    await nextTick();

    press(host, 'min', 'decrease');
    await nextTick();

    expect(ranges).toEqual([]);
  });
});
