// @vitest-environment jsdom
import { defineComponent, h, nextTick } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';


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

import { mount } from '#testing/mount';

const AboutPanel = (await import('./AboutPanel.vue')).default;

async function mountPanel(): Promise<HTMLElement> {
  const { host } = mount(AboutPanel);
  await nextTick();
  return host;
}

afterEach(() => {
  opened.urls.length = 0;
});

describe('AboutPanel font licences', () => {
  it('lists every bundled font with a link to its own licence file', async () => {
    const host = await mountPanel();

    const items = [...host.querySelectorAll('.font-license-item')];
    expect(items.map((item) => item.querySelector('strong')?.textContent)).toEqual([
      'Noto Serif TC',
      'Noto Sans TC',
      'Noto Serif SC',
      'Noto Sans SC'
    ]);

    const licences = items.map(
      (item) => item.querySelectorAll<HTMLAnchorElement>('a.setting-link')[1]?.getAttribute('href')
    );
    expect(licences).toEqual([
      '/licenses/noto-serif-tc-OFL-1.1.txt',
      '/licenses/noto-sans-tc-OFL-1.1.txt',
      '/licenses/noto-serif-sc-OFL-1.1.txt',
      '/licenses/noto-sans-sc-OFL-1.1.txt'
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

describe('AboutPanel third-party notices', () => {
  it('opens the generated third-party notice in-app', async () => {
    const host = await mountPanel();
    const link = host.querySelector<HTMLAnchorElement>('a[href="/licenses/third-party.txt"]');
    expect(link).not.toBeNull();

    link?.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    await nextTick();

    expect(host.querySelector('.font-license-modal')?.textContent).toBe('Third-party software licenses');
    expect(opened.urls).toEqual([]);
  });
});
