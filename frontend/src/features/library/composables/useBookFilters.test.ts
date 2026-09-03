// @vitest-environment jsdom
import { createApp, defineComponent, type App } from 'vue';
import { createMemoryHistory, createRouter, type Router } from 'vue-router';
import { afterEach, describe, expect, it } from 'vitest';
import { useBookFilters } from './useBookFilters';

// The filter panel had two end-to-end cases: one watched two chips, a removal
// and the URL stay in agreement, the other read the trigger's active-condition
// badge. Both are this composable's output — the panel only renders it — and
// neither needs a server to answer, since a condition is active or not by what
// the URL says. `apply.test.ts` covers which condition gets blamed for an empty
// list, and `filterLabels.test.ts` the words a chip is given.

let active: { app: App } | null = null;

async function mountFilters(query: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/books', component: defineComponent({ render: () => null }) }]
  });
  await router.push(`/books${query}`);
  await router.isReady();

  let api!: ReturnType<typeof useBookFilters>;
  const app = createApp(
    defineComponent({
      setup() {
        api = useBookFilters();
        return () => null;
      }
    })
  );
  app.use(router);
  app.mount(document.createElement('div'));
  active = { app };

  return { api, router };
}

/** Lets the composable's `router.push` settle before the query is read back. */
async function settle(): Promise<void> {
  await new Promise((done) => setTimeout(done, 0));
}

afterEach(() => {
  active?.app.unmount();
  active = null;
});

describe('useBookFilters', () => {
  it('counts no active condition on a bare library URL', async () => {
    const { api } = await mountFilters('');

    expect(api.activeCount.value).toBe(0);
    expect(api.hasActive.value).toBe(false);
    expect(api.chips.value).toEqual([]);
  });

  it('raises one chip per condition, whatever control shape it came from', async () => {
    // Cover is tri-state and author is a single facet, so this also proves the
    // chip row is driven by the registry rather than by a hand-kept list.
    const { api } = await mountFilters('?cover=none&author=none');

    expect(api.activeCount.value).toBe(2);
    expect(api.chips.value.map((chip) => chip.key).sort()).toEqual(['author', 'cover']);
    expect(api.chips.value.map((chip) => chip.label)).toEqual(
      expect.arrayContaining(['Cover: Unset', 'Author: Unset'])
    );
  });

  it('removes one condition from the URL and leaves the other alone', async () => {
    const { api, router } = await mountFilters('?cover=none&author=none');

    api.chips.value.find((chip) => chip.key === 'author')!.remove();
    await settle();

    expect(router.currentRoute.value.query.author).toBeUndefined();
    expect(router.currentRoute.value.query.cover).toBe('none');
    expect(api.activeCount.value).toBe(1);
    expect(api.chips.value.map((chip) => chip.key)).toEqual(['cover']);
  });

  it('clears every condition at once and returns to page 1', async () => {
    const { api, router } = await mountFilters('?cover=none&author=none&page=3');

    api.clearAll();
    await settle();

    expect(router.currentRoute.value.query.cover).toBeUndefined();
    expect(router.currentRoute.value.query.author).toBeUndefined();
    expect(router.currentRoute.value.query.page).toBe('1');
    expect(api.chips.value).toEqual([]);
  });

  it('ignores an unrelated query key rather than counting it as a condition', async () => {
    // The library's own sort and paging live in the same query string.
    const { api } = await mountFilters('?sort=title&order=asc&page=2');

    expect(api.activeCount.value).toBe(0);
  });
});
