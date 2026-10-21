// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NsfwPanel from './NsfwPanel.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('NsfwPanel', () => {
  it('reflects the current value and emits the next one as a boolean', () => {
    const onChange = vi.fn();
    const { host } = mount(NsfwPanel, { props: { value: false, disabled: false, onChange } });

    const control = host.querySelector<HTMLElement>('[role="switch"]');
    expect(control?.getAttribute('aria-checked')).toBe('false');
    control?.click();

    expect(onChange).toHaveBeenCalledExactlyOnceWith(true);
  });

  it('says where the marks come from, outside the row the switch controls', () => {
    const { host } = mount(NsfwPanel, { props: { value: false, disabled: false } });

    const note = host.querySelector('.settings-note');
    expect(note?.textContent).toContain('shelf.json');
    expect(host.querySelector('label.setting-item')?.contains(note!)).toBe(false);
  });
});
