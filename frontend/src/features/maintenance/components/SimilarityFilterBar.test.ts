// @vitest-environment jsdom
import { createApp, h, nextTick, reactive, type App } from 'vue';
import { afterEach, describe, expect, it } from 'vitest';

import SimilarityFilterBar from './SimilarityFilterBar.vue';
import {
  SIMILARITY_SLIDER_MAX,
  SIMILARITY_SLIDER_MIN,
  SIMILARITY_SLIDER_STEP,
  type SimilarityTierKey
} from '@/utils/similarity';

// Reka UI 2.10.5 SliderThumb uses ResizeObserver through useSize; jsdom 30.1.1 does not implement ResizeObserver.
class NoopResizeObserver {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
globalThis.ResizeObserver ??= NoopResizeObserver as unknown as typeof ResizeObserver;

interface Harness {
  app: App;
  host: HTMLElement;
  state: {
    tier: SimilarityTierKey;
    advancedOpen: boolean;
    threshold: number;
    subsetOnly: boolean;
  };
  events: { tier: SimilarityTierKey[]; advancedOpen: boolean[]; threshold: number[] };
}

const mounted: Harness[] = [];

function mount(overrides: Partial<Harness['state']> = {}): Harness {
  const host = document.createElement('div');
  document.body.append(host);

  const state = reactive({
    tier: 'same-book' as SimilarityTierKey,
    advancedOpen: false,
    threshold: 0.45,
    subsetOnly: false,
    ...overrides
  });
  const events: Harness['events'] = { tier: [], advancedOpen: [], threshold: [] };

  const app = createApp({
    setup: () => () =>
      h(SimilarityFilterBar, {
        tier: state.tier,
        advancedOpen: state.advancedOpen,
        threshold: state.threshold,
        subsetOnly: state.subsetOnly,
        'onUpdate:tier': (value: SimilarityTierKey) => {
          events.tier.push(value);
          state.tier = value;
        },
        'onUpdate:advancedOpen': (value: boolean) => {
          events.advancedOpen.push(value);
          state.advancedOpen = value;
        },
        'onUpdate:threshold': (value: number) => {
          events.threshold.push(value);
          state.threshold = value;
        }
      })
  });
  app.mount(host);

  const entry: Harness = { app, host, state, events };
  mounted.push(entry);
  return entry;
}

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

function segments(host: HTMLElement): HTMLButtonElement[] {
  return Array.from(host.querySelectorAll<HTMLButtonElement>('.similarity-segment'));
}

function pressed(host: HTMLElement): string[] {
  return segments(host)
    .filter((button) => button.getAttribute('aria-pressed') === 'true')
    .map((button) => button.textContent?.trim() ?? '');
}

function advancedToggle(host: HTMLElement): HTMLButtonElement {
  const button = host.querySelector<HTMLButtonElement>('.similarity-advanced-toggle');
  if (!button) throw new Error('the advanced toggle is missing');
  return button;
}

describe('SimilarityFilterBar segmented tiers', () => {
  it('marks exactly the selected tier as pressed', () => {
    const { host } = mount({ tier: 'same-book' });

    expect(segments(host)).toHaveLength(3);
    expect(pressed(host)).toHaveLength(1);
    const active = segments(host).find((button) => button.getAttribute('aria-pressed') === 'true');
    expect(active?.getAttribute('data-state')).toBe('on');
  });

  it('emits the picked tier when a different segment is chosen', async () => {
    const { host, events } = mount({ tier: 'same-book' });

    const other = segments(host).find((button) => button.getAttribute('aria-pressed') !== 'true');
    other?.click();
    await nextTick();

    expect(events.tier).toHaveLength(1);
    expect(pressed(host)).toHaveLength(1);
  });

  it('keeps the selection when the active segment is re-clicked', async () => {
    const { host, events } = mount({ tier: 'same-book' });

    const active = segments(host).find((button) => button.getAttribute('aria-pressed') === 'true');
    active?.click();
    await nextTick();

    expect(events.tier).toHaveLength(0);
    expect(pressed(host)).toHaveLength(1);
  });

  it('presses no tier while the advanced slider owns the selection', () => {
    const { host } = mount({ advancedOpen: true });

    expect(pressed(host)).toHaveLength(0);
    for (const button of segments(host)) {
      expect(button.getAttribute('data-state')).toBe('off');
    }
  });

  it('stays controlled when mounted with the advanced slider open', async () => {
    const { host, events } = mount({ advancedOpen: true });
    expect(pressed(host)).toHaveLength(0);

    segments(host)[0].click();
    await nextTick();
    expect(events.tier).toHaveLength(1);
    expect(pressed(host)).toHaveLength(1);

    const active = segments(host).find((button) => button.getAttribute('aria-pressed') === 'true');
    active?.click();
    await nextTick();
    expect(events.tier).toHaveLength(1);
    expect(pressed(host)).toHaveLength(1);
  });
});

describe('SimilarityFilterBar advanced collapsible', () => {
  it('reflects the open state on the trigger and reveals the slider only when open', async () => {
    const { host } = mount({ advancedOpen: false });

    expect(advancedToggle(host).getAttribute('aria-expanded')).toBe('false');
    expect(host.querySelector('#similarity-threshold-slider')).toBeNull();

    const { host: openHost } = mount({ advancedOpen: true });
    await nextTick();
    expect(advancedToggle(openHost).getAttribute('aria-expanded')).toBe('true');
    expect(openHost.querySelector('#similarity-threshold-slider')).not.toBeNull();
  });

  it('emits the toggled open state when the trigger is clicked', async () => {
    const { host, events } = mount({ advancedOpen: false });

    advancedToggle(host).click();
    await nextTick();

    expect(events.advancedOpen).toEqual([true]);
  });
});

function thumb(host: HTMLElement): HTMLElement {
  const element = host.querySelector<HTMLElement>('#similarity-threshold-slider');
  if (!element) throw new Error('the slider thumb is missing');
  return element;
}

describe('SimilarityFilterBar threshold slider', () => {
  it('exposes the threshold on a slider the label names', async () => {
    const { host } = mount({ advancedOpen: true, threshold: 0.45 });
    await nextTick();

    const control = thumb(host);
    expect(control.getAttribute('role')).toBe('slider');
    expect(control.getAttribute('aria-valuenow')).toBe('0.45');
    expect(control.getAttribute('aria-valuemin')).toBe(String(SIMILARITY_SLIDER_MIN));
    expect(control.getAttribute('aria-valuemax')).toBe(String(SIMILARITY_SLIDER_MAX));

    const labelId = control.getAttribute('aria-labelledby');
    expect(labelId).toBeTruthy();
    expect(host.querySelector(`#${labelId}`)?.textContent?.trim()).toBeTruthy();
  });

  it('steps the threshold by SIMILARITY_SLIDER_STEP with an arrow key', async () => {
    const { host, events, state } = mount({ advancedOpen: true, threshold: 0.45 });
    await nextTick();

    thumb(host).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    await nextTick();

    expect(events.threshold).toEqual([0.45 + SIMILARITY_SLIDER_STEP]);
    expect(state.threshold).toBe(0.46);
    expect(thumb(host).getAttribute('aria-valuenow')).toBe('0.46');

    thumb(host).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    await nextTick();

    expect(state.threshold).toBe(0.45);
  });

  it('does not step past the ends of the range', async () => {
    const { host, state } = mount({ advancedOpen: true, threshold: SIMILARITY_SLIDER_MAX });
    await nextTick();

    thumb(host).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    await nextTick();

    expect(state.threshold).toBe(SIMILARITY_SLIDER_MAX);
  });
});
