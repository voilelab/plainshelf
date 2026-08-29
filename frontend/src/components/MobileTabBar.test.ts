// @vitest-environment jsdom
import { createApp, defineComponent, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The bar renders RouterLinks; a real router is more than this contract needs,
// so stub RouterLink to echo the props that define each tab — its target and
// which active-class strategy it uses — onto data-attributes.
vi.mock('vue-router', () => ({
  RouterLink: defineComponent({
    props: {
      to: { type: [String, Object], default: '' },
      activeClass: { type: String, default: undefined },
      exactActiveClass: { type: String, default: undefined }
    },
    setup(props, { slots }) {
      return () =>
        h(
          'a',
          {
            'data-to': typeof props.to === 'string' ? props.to : JSON.stringify(props.to),
            'data-active-class': props.activeClass ?? '',
            'data-exact-active-class': props.exactActiveClass ?? ''
          },
          slots.default?.()
        );
    }
  })
}));

import MobileTabBar from './MobileTabBar.vue';
import { setLocale } from '@/i18n';

const mounted: Array<{ app: App; host: HTMLElement }> = [];

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(MobileTabBar);
  app.mount(host);
  mounted.push({ app, host });
  return host;
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

describe('MobileTabBar', () => {
  it('renders the four mobile destinations in order', () => {
    const tabs = [...mount().querySelectorAll<HTMLElement>('.mobile-tab')];

    expect(tabs.map((tab) => tab.getAttribute('data-to'))).toEqual([
      '/books',
      '/read-history',
      '/downloads',
      '/settings'
    ]);
    expect(tabs.map((tab) => tab.querySelector('.mobile-tab-label')?.textContent)).toEqual([
      'Library',
      'Recently Read',
      'Downloads',
      'Settings'
    ]);
  });

  it('lights the library tab on prefix match and the rest only on exact match', () => {
    const tabs = [...mount().querySelectorAll<HTMLElement>('.mobile-tab')];
    const byTarget = (to: string) => tabs.find((tab) => tab.getAttribute('data-to') === to)!;

    // Library covers /books and every book detail below it: prefix match.
    expect(byTarget('/books').getAttribute('data-active-class')).toBe('active');
    expect(byTarget('/books').getAttribute('data-exact-active-class')).toBe('');

    // The leaf routes must not stay lit on descendants: exact match only.
    for (const to of ['/read-history', '/downloads', '/settings']) {
      expect(byTarget(to).getAttribute('data-active-class')).toBe('');
      expect(byTarget(to).getAttribute('data-exact-active-class')).toBe('active');
    }
  });
});
