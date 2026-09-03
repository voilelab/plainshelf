// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

// The bundled fonts ship under the OFL, so the licence text has to be reachable
// from the app itself. The end-to-end reader-font case used to check this list
// on its way past Settings; it is a static render, so it belongs here.

vi.mock('@/api/version', () => ({ getServerVersion: () => Promise.resolve('1.2.3') }));

const opened = vi.hoisted(() => ({ urls: [] as string[] }));
vi.mock('@/features/settings/utils/externalLinks', () => ({
  openExternalURL: (url: string) => {
    opened.urls.push(url);
    return Promise.resolve();
  }
}));

vi.mock('@/features/settings/components/FontLicenseModal.vue', () => ({
  default: defineComponent({
    props: { open: { type: Boolean, default: false }, title: { type: String, default: '' } },
    setup: (props) => () =>
      props.open ? h('div', { class: 'font-license-modal' }, props.title) : null
  })
}));

const AboutPanel = (await import('./AboutPanel.vue')).default;

let active: { app: App; host: HTMLElement } | null = null;

async function mountPanel(): Promise<HTMLElement> {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(AboutPanel) }));
  app.mount(host);
  active = { app, host };
  await nextTick();
  return host;
}

afterEach(() => {
  active?.app.unmount();
  active?.host.remove();
  active = null;
  opened.urls.length = 0;
});

describe('AboutPanel font licences', () => {
  it('lists every bundled font with a link to its own licence file', async () => {
    const host = await mountPanel();

    const items = [...host.querySelectorAll('.font-license-item')];
    expect(items.map((item) => item.querySelector('strong')?.textContent)).toEqual([
      'Noto Serif TC',
      'Noto Sans TC'
    ]);

    // One licence per font, not one shared file: the two faces are separate
    // downloads with separate copyright lines.
    const licences = items.map(
      (item) => item.querySelectorAll<HTMLAnchorElement>('a.setting-link')[1]?.getAttribute('href')
    );
    expect(licences).toEqual([
      '/licenses/noto-serif-tc-OFL-1.1.txt',
      '/licenses/noto-sans-tc-OFL-1.1.txt'
    ]);
    expect(new Set(licences).size).toBe(licences.length);
  });

  it('opens the licence in-app rather than navigating away from Settings', async () => {
    const host = await mountPanel();
    const licenceLink = host.querySelectorAll<HTMLAnchorElement>(
      '.font-license-item a.setting-link'
    )[1];

    licenceLink.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    await nextTick();

    expect(host.querySelector('.font-license-modal')).not.toBeNull();
    // The href stays a real path for copy-link and no-JS reachability, but the
    // click is handled here, so nothing is handed to the shell's browser.
    expect(opened.urls).toEqual([]);
  });

  it('hands an upstream source link to the shell instead of the WebView', async () => {
    const host = await mountPanel();
    const sourceLink = host.querySelector<HTMLAnchorElement>('.font-license-item a.setting-link');

    sourceLink?.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    await nextTick();

    expect(opened.urls).toEqual(['https://fontsource.org/fonts/noto-serif-tc']);
  });
});
