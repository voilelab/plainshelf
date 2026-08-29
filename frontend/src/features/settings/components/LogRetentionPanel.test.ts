// @vitest-environment jsdom
import { createApp } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import LogRetentionPanel from './LogRetentionPanel.vue';
import { setLocale } from '@/i18n';

function mount(value: number) {
  const host = document.createElement('div');
  const app = createApp(LogRetentionPanel, { value, disabled: false });
  app.mount(host);
  return { host, app };
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

    const input = host.querySelector<HTMLInputElement>('input[type="number"]');
    expect(input?.min).toBe('0');
    expect(input?.max).toBe('3650');

    app.unmount();
  });
});
