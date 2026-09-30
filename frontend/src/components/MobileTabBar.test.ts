// @vitest-environment jsdom
import { createApp, defineComponent, h, type App } from 'vue';
import { createRouter, createMemoryHistory, type Router } from 'vue-router';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import MobileTabBar from './MobileTabBar.vue';
import { setLocale } from '@/i18n';

// Mirror the sibling shape of the real router (router.ts): `library` and
// `book-detail` are separate records, not nested, which is exactly why the
// Library tab cannot rely on RouterLink's record-based active matching.
const routes = [
  { path: '/', redirect: '/books' },
  { path: '/books', name: 'library', component: { template: '<div />' } },
  { path: '/books/:id', name: 'book-detail', component: { template: '<div />' } },
  { path: '/read-history', name: 'read-history', component: { template: '<div />' } },
  { path: '/downloads', name: 'downloads', component: { template: '<div />' } },
  { path: '/settings', name: 'settings', component: { template: '<div />' } }
];

const mounted: Array<{ app: App; host: HTMLElement }> = [];

async function mountAt(path: string): Promise<HTMLElement> {
  const router: Router = createRouter({ history: createMemoryHistory(), routes });
  await router.push(path);
  await router.isReady();

  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(MobileTabBar) }));
  app.use(router);
  app.mount(host);
  mounted.push({ app, host });
  return host;
}

function activeTargets(host: HTMLElement): string[] {
  return [...host.querySelectorAll<HTMLElement>('.mobile-tab.active')].map(
    (tab) => tab.getAttribute('href') ?? ''
  );
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
  it('renders the four mobile destinations in order', async () => {
    const tabs = [...(await mountAt('/books')).querySelectorAll<HTMLElement>('.mobile-tab')];

    expect(tabs.map((tab) => tab.getAttribute('href'))).toEqual([
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

  it('keeps the Library tab active on the list, a book, and a filtered view', async () => {
    expect(activeTargets(await mountAt('/books'))).toEqual(['/books']);
    // The sibling book-detail record is the case the stubbed test missed and
    // the Codex review caught: the Library tab must still light up here.
    expect(activeTargets(await mountAt('/books/abc123'))).toEqual(['/books']);
    expect(activeTargets(await mountAt('/books?author=none'))).toEqual(['/books']);
  });

  it('activates a leaf tab only on its own route, never on a sibling', async () => {
    expect(activeTargets(await mountAt('/read-history'))).toEqual(['/read-history']);
    expect(activeTargets(await mountAt('/downloads'))).toEqual(['/downloads']);
    expect(activeTargets(await mountAt('/settings'))).toEqual(['/settings']);
    // A book route must not light Recently read / Downloads / Settings.
    expect(activeTargets(await mountAt('/books/abc123'))).toEqual(['/books']);
  });
});
