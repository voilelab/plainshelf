// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App, type Component } from 'vue';
import { createMemoryHistory, createRouter, RouterView } from 'vue-router';
import { afterEach, describe, expect, it } from 'vitest';
import SeriesDetailPage from './SeriesDetailPage.vue';
import SeriesListPage from './SeriesListPage.vue';
import { useBookStore } from '@/composables/useBookStore';
import { setLocale } from '@/i18n';
import type { Book } from '@/types/book';

function book(title: string, series?: Book['series']): Book {
  return { id: title, title, authors: [], tags: [], folders: [], series };
}

let mounted: { app: App; host: HTMLElement } | null = null;
let activeRouter: ReturnType<typeof createRouter> | null = null;

async function mountAt(path: string, books: Book[], loaded = true): Promise<HTMLElement> {
  setLocale('en');
  useBookStore().books.value = books;
  useBookStore().loaded.value = loaded;
  const stub = defineComponent({ render: () => null });
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/series', name: 'series-list', component: SeriesListPage as Component },
      { path: '/series/:name', name: 'series-detail', component: SeriesDetailPage as Component, props: true },
      { path: '/:rest(.*)*', component: stub }
    ]
  });
  await router.push(path);
  await router.isReady();
  activeRouter = router;

  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(RouterView) });
  app.use(router);
  app.mount(host);
  mounted = { app, host };
  await nextTick();
  return host;
}

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
  useBookStore().books.value = [];
  useBookStore().loaded.value = false;
});

const SHELF = [
  book('Vol 2', { name: 'Saga', index: 2 }),
  book('Side Story', { name: 'Saga', index: 1.5 }),
  book('Vol 1', { name: 'Saga', index: 1 }),
  book('Atlas', { name: 'Atlas', index: 1 }),
  book('Loose')
];

describe('SeriesListPage', () => {
  it('lists each series with its book count, linking to its page', async () => {
    const host = await mountAt('/series', SHELF);

    const rows = Array.from(host.querySelectorAll<HTMLAnchorElement>('.series-row'));
    expect(
      rows.map((row) => `${row.querySelector('.series-name')?.textContent}|${row.querySelector('.series-books')?.textContent}`)
    ).toEqual(['Atlas|1 book', 'Saga|3 books']);
    expect(rows.map((row) => row.getAttribute('href'))).toEqual(['/series/Atlas', '/series/Saga']);
  });

  it('shows a cover thumbnail on every series row', async () => {
    const host = await mountAt('/series', SHELF);

    const covers = Array.from(host.querySelectorAll<HTMLImageElement>('.series-row img.series-cover'));
    expect(covers).toHaveLength(2);
    expect(covers.every((img) => img.getAttribute('src'))).toBe(true);
  });

  it('explains how to start a series on a shelf without one', async () => {
    const host = await mountAt('/series', [book('Loose')]);

    expect(host.querySelector('.series-row')).toBeNull();
    expect(host.querySelector('.empty-state')?.textContent).toContain('No series yet.');
  });
});

describe('SeriesDetailPage', () => {
  it('lists the series books in number order', async () => {
    const host = await mountAt('/series/Saga', SHELF);

    expect(host.querySelector('.bookshelf-title')?.textContent?.trim()).toBe('Saga');
    const text = host.querySelector('.bookshelf-content')?.textContent ?? '';
    const order = ['Vol 1', 'Side Story', 'Vol 2'].map((title) => text.indexOf(title));
    expect(order.every((at) => at >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    expect(text).not.toContain('Atlas');
    expect(text).not.toContain('Loose');
  });

  it('shows an empty state for a series no book belongs to', async () => {
    const host = await mountAt('/series/Nothing', SHELF);

    expect(host.querySelector('.empty-state')?.textContent).toContain('No book on this shelf belongs to this series.');
  });

  it('keeps the page in the address and shows loading until the shelf has loaded', async () => {
    const host = await mountAt('/series/Saga?page=3', [], false);
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(activeRouter?.currentRoute.value.query.page).toBe('3');
    expect(host.querySelector('.empty-state')).toBeNull();
    expect(host.querySelector('.loading')).not.toBeNull();

    useBookStore().books.value = Array.from({ length: 120 }, (_, i) => book(`Vol ${i}`, { name: 'Saga', index: i }));
    useBookStore().loaded.value = true;
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(activeRouter?.currentRoute.value.query.page).toBe('3');
    expect(host.querySelector('.loading')).toBeNull();
  });
});
