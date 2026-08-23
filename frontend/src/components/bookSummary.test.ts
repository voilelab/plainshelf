// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import BookCardView from './BookCardView.vue';
import BookListView from './BookListView.vue';
import en from '@/i18n/locales/en';
import type { Book } from '@/types/book';

/**
 * The card and the list show the same book's description in the same words, so
 * they are asserted against each other rather than one at a time. Rendering
 * them through `vue/server-renderer` - which ships with Vue - keeps that
 * possible without a component-test harness the repository does not have.
 */
function book(overrides: Partial<Book>): Book {
  return { id: 'b1', title: '書名', authors: [], tags: [], folders: [], ...overrides };
}

async function render(component: typeof BookCardView | typeof BookListView, value: Book): Promise<Document> {
  const html = await renderToString(createSSRApp(component, { books: [value] }));
  return new DOMParser().parseFromString(html, 'text/html');
}

async function summaries(value: Book): Promise<{ card: string | null; list: string | null }> {
  const [card, list] = await Promise.all([render(BookCardView, value), render(BookListView, value)]);
  return {
    card: card.querySelector('.book-card-summary')?.textContent ?? null,
    list: list.querySelector('.book-list-comment')?.textContent ?? null
  };
}

describe('book summaries', () => {
  it('shows the description as words, identically in both views', async () => {
    const comment = '<h1>書名</h1>\n<p>第一段</p><ul><li>一</li><li>二</li></ul>';
    const { card, list } = await summaries(book({ comment }));

    expect(card).toBe('書名 第一段 一 二');
    expect(list).toBe(card);
  });

  it('leaves no markup or Markdown syntax on either view', async () => {
    const comment = '# 標題\n\n**粗體**\n\n<p>段落</p>';
    const { card, list } = await summaries(book({ comment }));

    for (const summary of [card, list]) {
      expect(summary).not.toMatch(/[<>*#]|[\r\n]/);
    }
    expect(list).toBe(card);
  });

  it('falls back when the description strips down to nothing', async () => {
    for (const comment of ['<br>', '<p></p>', '![封面](cover.png)']) {
      const { card, list } = await summaries(book({ comment, authors: ['作者甲', '作者乙'] }));

      expect(card, comment).toBe('作者甲, 作者乙');
      expect(list, comment).toBeNull();
    }
  });

  it('says so when a book has neither a description nor an author', async () => {
    const { card, list } = await summaries(book({ comment: '<p>  </p>' }));

    expect(card).toBe(en.bookCollection.noSummary);
    expect(list).toBeNull();
  });
});
