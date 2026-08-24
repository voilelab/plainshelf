// @vitest-environment jsdom
import { createApp, defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import TagCloud from './TagCloud.vue';
import { setLocale } from '@/i18n';

// RouterLink is registered globally by the app; stub it so each chip's target is
// present and inspectable without standing up a router.
const RouterLinkStub = defineComponent({
  props: { to: { type: [String, Object], default: '' } },
  setup(props, { slots }) {
    return () =>
      h('a', { class: 'router-link-stub', 'data-to': JSON.stringify(props.to) }, slots.default?.());
  }
});

function mount(tagCounts: Record<string, number>) {
  const host = document.createElement('div');
  const app = createApp(TagCloud, { tagCounts });
  app.component('RouterLink', RouterLinkStub);
  app.mount(host);
  return { host, app };
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  // jsdom host is discarded per test; nothing else to reset.
});

describe('TagCloud', () => {
  it('renders each tag as a link that applies that tag on the library page', () => {
    const { host, app } = mount({ fiction: 3, 'sci-fi': 1 });

    const links = [...host.querySelectorAll<HTMLAnchorElement>('.tag-chip')];
    expect(links).toHaveLength(2);

    const targets = links.map((link) => JSON.parse(link.getAttribute('data-to') ?? '{}'));
    // The higher count sorts first; both land on /books with the tags filter
    // encoded as the registry's `eq:` token plus the `all` combinator, so the
    // library parses them back into an active filter rather than ignoring them.
    expect(targets).toEqual([
      { path: '/books', query: { tags: 'eq:fiction', tagsOp: 'all' } },
      { path: '/books', query: { tags: 'eq:sci-fi', tagsOp: 'all' } }
    ]);

    app.unmount();
  });

  it('shows the empty message and no links when there are no tags', () => {
    const { host, app } = mount({});

    expect(host.querySelector('.tag-chip')).toBeNull();
    expect(host.querySelector('.tag-cloud-empty')?.textContent).toContain('No tags yet');

    app.unmount();
  });
});
