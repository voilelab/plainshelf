// @vitest-environment jsdom
import { nextTick } from 'vue';
import { describe, expect, it } from 'vitest';

import { getPreference } from '@/storage/preferences';
import ImportNotificationsPanel from './ImportNotificationsPanel.vue';
import { mount } from '#testing/mount';

describe('ImportNotificationsPanel', () => {
  it('is on by default and stores false when switched off', async () => {
    const { host } = mount(ImportNotificationsPanel);
    const toggle = host.querySelector<HTMLButtonElement>('button[role="switch"]');
    expect(toggle?.getAttribute('aria-checked')).toBe('true');

    toggle?.click();
    await nextTick();

    expect(toggle?.getAttribute('aria-checked')).toBe('false');
    expect(getPreference('import-notifications')).toBe('false');
  });
});
