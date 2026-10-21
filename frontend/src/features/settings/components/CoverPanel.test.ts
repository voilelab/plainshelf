// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import CoverPanel from './CoverPanel.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('CoverPanel', () => {
  it('renders the setting as a switch that reflects the current value', () => {
    const { host } = mount(CoverPanel, { props: { value: true, disabled: false } });

    const control = host.querySelector('[role="switch"]');
    expect(control).not.toBeNull();
    expect(control?.getAttribute('aria-checked')).toBe('true');
    expect(host.querySelector('input[type="checkbox"]')).toBeNull();
  });

  it('emits the next value as a boolean rather than a DOM event', () => {
    const onChange = vi.fn();
    const { host } = mount(CoverPanel, { props: { value: false, disabled: false, onChange } });

    host.querySelector<HTMLElement>('[role="switch"]')?.click();

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('names the switch from the title alone, not the whole row', () => {
    const { host } = mount(CoverPanel, { props: { value: false, disabled: false } });

    const control = host.querySelector('[role="switch"]');
    const title = host.querySelector('.setting-label');
    const description = host.querySelector('p.setting-description');

    expect(host.querySelector('label.setting-item')?.getAttribute('for')).toBe(control?.id);
    expect(control?.getAttribute('aria-labelledby')).toBe(title?.id);
    expect(control?.getAttribute('aria-describedby')).toBe(description?.id);
  });

  it('keeps the whole row a click target, description included', () => {
    const onChange = vi.fn();
    const { host } = mount(CoverPanel, { props: { value: false, disabled: false, onChange } });

    host.querySelector<HTMLElement>('p.setting-description')?.click();
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(true);

    host.querySelector<HTMLElement>('.setting-label')?.click();
    expect(onChange).toHaveBeenCalledTimes(2);
  });

  it('fires once when the switch itself is clicked inside the row label', () => {
    const onChange = vi.fn();
    const { host } = mount(CoverPanel, { props: { value: false, disabled: false, onChange } });

    host.querySelector<HTMLElement>('[role="switch"]')?.click();

    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it('does not emit while the panel is saving', () => {
    const onChange = vi.fn();
    const { host } = mount(CoverPanel, { props: { value: false, disabled: true, onChange } });

    const control = host.querySelector<HTMLButtonElement>('[role="switch"]');
    expect(control?.disabled).toBe(true);
    control?.click();

    expect(onChange).not.toHaveBeenCalled();
  });
});
