// @vitest-environment jsdom

import { createApp, defineComponent, ref, type App, type Ref } from 'vue';
import { createMemoryHistory, createRouter, type Router } from 'vue-router';
import { afterEach, describe, expect, it } from 'vitest';
import { countPages, pageSlice, useBookCollectionRoute } from './useBookCollectionRoute';
import { setPageSize } from './useBookPagination';

describe('countPages', () => {
  it('rounds up a partial last page', () => {
    expect(countPages(101, 50)).toBe(3);
    expect(countPages(100, 50)).toBe(2);
  });

  it('stays at 1 for an empty list, so page 1 is always valid', () => {
    expect(countPages(0, 50)).toBe(1);
  });

  it('gives every item its own page when the page size is 1', () => {
    expect(countPages(7, 1)).toBe(7);
  });
});

describe('pageSlice', () => {
  const items = Array.from({ length: 10 }, (_, index) => index);

  it('returns the window for the requested 1-based page', () => {
    expect(pageSlice(items, 1, 3)).toEqual([0, 1, 2]);
    expect(pageSlice(items, 2, 3)).toEqual([3, 4, 5]);
  });

  it('returns a short final page', () => {
    expect(pageSlice(items, 4, 3)).toEqual([9]);
  });

  it('returns nothing past the end rather than throwing', () => {
    expect(pageSlice(items, 99, 3)).toEqual([]);
  });

  it('does not mutate the source list', () => {
    const source = [...items];
    pageSlice(source, 2, 3);
    expect(source).toEqual(items);
  });
});

// The composable is exercised through a real router and a mounted host so the
// clamp watcher runs the way it does on a page. There is no component-mounting
// helper in this project, so the harness below is deliberately minimal.
interface TrashedRow {
  id: string;
  title: string;
}

function rows(count: number, offset = 0): TrashedRow[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `id-${offset + index + 1}`,
    title: `title-${offset + index + 1}`
  }));
}

let active: { app: App; router: Router } | null = null;

async function mountCollection<T>(options: {
  items: Ref<T[]>;
  initialPath: string;
  clampEnabled?: Ref<boolean>;
}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/trash', component: defineComponent({ render: () => null }) }]
  });
  await router.push(options.initialPath);
  await router.isReady();

  let api!: ReturnType<typeof useBookCollectionRoute<T>>;
  const Host = defineComponent({
    setup() {
      api = useBookCollectionRoute<T>({
        items: options.items,
        buildQuery: (page) => ({ ...router.currentRoute.value.query, page: String(page) }),
        clampEnabled: options.clampEnabled
      });
      return () => null;
    }
  });

  const app = createApp(Host);
  app.use(router);
  app.mount(document.createElement('div'));
  active = { app, router };

  // Let the immediate clamp watcher's router.replace settle.
  await router.isReady();
  await new Promise((resolve) => setTimeout(resolve, 0));

  return { api, router };
}

describe('useBookCollectionRoute', () => {
  afterEach(() => {
    active?.app.unmount();
    active = null;
    setPageSize(50);
  });

  it('clamps an out-of-range page immediately when no gate is given', async () => {
    setPageSize(10);
    const items = ref<TrashedRow[]>([]);

    const { router } = await mountCollection({ items, initialPath: '/trash?page=3' });

    expect(router.currentRoute.value.query.page).toBe('1');
  });

  it('keeps a deep-linked page while the gate is closed, then slices once the list arrives', async () => {
    setPageSize(10);
    const items = ref<TrashedRow[]>([]);
    const loaded = ref(false);

    const { api, router } = await mountCollection({
      items,
      initialPath: '/trash?page=3',
      clampEnabled: loaded
    });

    expect(router.currentRoute.value.query.page).toBe('3');

    items.value = rows(25);
    loaded.value = true;
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(router.currentRoute.value.query.page).toBe('3');
    expect(api.totalPages.value).toBe(3);
    expect(api.visibleBooks.value.map((row) => row.id)).toEqual(['id-21', 'id-22', 'id-23', 'id-24', 'id-25']);
  });

  it('clamps back into range when the list shrinks under the current page', async () => {
    setPageSize(10);
    const items = ref<TrashedRow[]>(rows(25));
    const loaded = ref(true);

    const { router } = await mountCollection({
      items,
      initialPath: '/trash?page=3',
      clampEnabled: loaded
    });

    expect(router.currentRoute.value.query.page).toBe('3');

    items.value = rows(5);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(router.currentRoute.value.query.page).toBe('1');
  });

  it('preserves unrelated query keys when the page changes', async () => {
    setPageSize(10);
    const items = ref<TrashedRow[]>(rows(25));
    const loaded = ref(true);

    const { api, router } = await mountCollection({
      items,
      initialPath: '/trash?page=1&sort=deleted',
      clampEnabled: loaded
    });

    api.onPageChange(2);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(router.currentRoute.value.query).toEqual({ page: '2', sort: 'deleted' });
  });
});
