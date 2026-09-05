// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import BookNsfwBadge from './BookNsfwBadge.vue';
import { setLocale } from '@/i18n';
import type { Book } from '@/types/book';

const mounted: App[] = [];

function mount(book: Pick<Book, 'nsfw' | 'nsfw_folder'>): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(BookNsfwBadge, { book }) });
  app.mount(host);
  mounted.push(app);
  return host;
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  for (const app of mounted.splice(0)) {
    app.unmount();
  }
  document.body.innerHTML = '';
});

describe('BookNsfwBadge', () => {
  it('renders for either half of the mark and for neither', () => {
    // The folder row is the one a card cannot get from `nsfw`: without it,
    // exactly the books a folder rule hides would look unmarked in the listing
    // that is showing them because the setting is on.
    const cases: [Pick<Book, 'nsfw' | 'nsfw_folder'>, boolean][] = [
      [{}, false],
      [{ nsfw: false }, false],
      [{ nsfw: true }, true],
      [{ nsfw: false, nsfw_folder: { path: 'Fiction/Adult' } }, true]
    ];

    for (const [book, shown] of cases) {
      expect(mount(book).querySelector('.book-nsfw-badge') !== null, JSON.stringify(book)).toBe(shown);
    }
  });

  it('carries a text label, not colour alone', () => {
    const badge = mount({ nsfw: true }).querySelector('.book-nsfw-badge');
    expect(badge?.textContent?.trim()).toBe('NSFW');
    expect(badge?.getAttribute('title')).toBe('Marked as adult content');
  });
});
