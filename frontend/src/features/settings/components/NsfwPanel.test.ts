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

// Only what is this panel's own. The switch's DOM contract — role, aria-checked,
// the row label pointing at it by id, the disabled button — belongs to
// BaseSwitch and the shared row markup, and CoverPanel.test.ts already pins it.
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

    // Outside the label, so it neither joins the switch's accessible
    // description nor becomes another way to toggle the setting.
    const note = host.querySelector('.settings-note');
    expect(note?.textContent).toContain('shelf.json');
    expect(host.querySelector('label.setting-item')?.contains(note!)).toBe(false);
  });
});
