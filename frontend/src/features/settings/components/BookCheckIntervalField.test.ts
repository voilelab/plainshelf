// @vitest-environment jsdom
import { createApp } from 'vue';
import { beforeEach, describe, expect, it } from 'vitest';

import BookCheckIntervalField from './BookCheckIntervalField.vue';
import { setLocale } from '@/i18n';

function mount(initial: string) {
  const host = document.createElement('div');
  const app = createApp(BookCheckIntervalField, { modelValue: initial });
  app.mount(host);
  return { host, app };
}

beforeEach(() => {
  setLocale('en');
});

describe('BookCheckIntervalField', () => {
  // The wrapper is only a set of copy keys over ScanIntervalField, so every
  // label it does not delegate announces the per-book control as the separate
  // shelf-wide scan setting.
  it('names the per-book interval on the amount box and its steppers', () => {
    const { host, app } = mount('10m');

    const amount = host.querySelector<HTMLInputElement>('[data-testid="book-check-interval-amount"]');
    expect(amount?.getAttribute('aria-label')).toBe('Per-book check interval amount');

    const steppers = [...host.querySelectorAll('.number-field-step')].map((el) =>
      el.getAttribute('aria-label')
    );
    expect(steppers).toEqual([
      'Decrease Per-book check interval amount',
      'Increase Per-book check interval amount'
    ]);

    app.unmount();
  });
});
