// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import ErrorIncidentNotice from './ErrorIncidentNotice.vue';
import { reportIncident, useErrorIncident } from '@/composables/useErrorIncident';
import { setLocale } from '@/i18n';

const { dismissIncident } = useErrorIncident();
const mounted: Array<{ app: App; host: HTMLElement }> = [];

function mountNotice(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(ErrorIncidentNotice) }));
  app.mount(host);
  mounted.push({ app, host });
  return host;
}

function notice(host: HTMLElement): HTMLElement | null {
  return host.querySelector('.error-incident');
}

// The copy handler awaits the clipboard write before flipping the label, so one
// tick lands before the promise it is waiting on has settled.
async function flush(): Promise<void> {
  await Promise.resolve();
  await nextTick();
}

beforeEach(() => {
  setLocale('en');
  dismissIncident();
});

afterEach(() => {
  for (const { app, host } of mounted.splice(0)) {
    app.unmount();
    host.remove();
  }
  vi.unstubAllGlobals();
});

describe('ErrorIncidentNotice', () => {
  // "No incident, no change to the layout": nothing is rendered at all, so the
  // notice cannot reserve space on a page that has no reference to show.
  it('renders nothing without a reference', () => {
    expect(notice(mountNotice())).toBeNull();
  });

  it('shows the reference once one is raised', async () => {
    const host = mountNotice();
    reportIncident('K7MQ4XZB');
    await nextTick();

    expect(notice(host)?.textContent).toContain('K7MQ4XZB');
    expect(notice(host)?.getAttribute('role')).toBe('status');
  });

  it('copies the reference to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });

    const host = mountNotice();
    reportIncident('c-K7MQ4XZB');
    await nextTick();

    host.querySelector<HTMLButtonElement>('.error-incident__copy')?.click();
    await flush();

    expect(writeText).toHaveBeenCalledWith('c-K7MQ4XZB');
    expect(notice(host)?.textContent).toContain('Copied');
  });

  it('reports the next reference as uncopied', async () => {
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });

    const host = mountNotice();
    reportIncident('c-K7MQ4XZB');
    await nextTick();
    host.querySelector<HTMLButtonElement>('.error-incident__copy')?.click();
    await flush();
    expect(notice(host)?.textContent).toContain('Copied');

    reportIncident('ABCD2345');
    await nextTick();

    expect(notice(host)?.textContent).toContain('Copy');
    expect(notice(host)?.textContent).not.toContain('Copied');
  });

  it('stays quiet when the browser has no clipboard', async () => {
    vi.stubGlobal('navigator', {});

    const host = mountNotice();
    reportIncident('ABCD2345');
    await nextTick();

    host.querySelector<HTMLButtonElement>('.error-incident__copy')?.click();
    await flush();

    expect(notice(host)?.textContent).not.toContain('Copied');
  });

  it('dismisses the reference', async () => {
    const host = mountNotice();
    reportIncident('ABCD2345');
    await nextTick();

    host.querySelector<HTMLButtonElement>('.error-incident__dismiss')?.click();
    await nextTick();

    expect(notice(host)).toBeNull();
  });
});
