// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('./FolderBreadcrumb.vue', () => ({
  default: { setup: () => () => h('nav') }
}));

import BookDetail from './BookDetail.vue';
import type { Book } from '@/types/book';
import type { SourceMeta } from '@/types/source';

function book(comment: string): Book {
  return { id: 'book-1', title: '書名', authors: [], tags: [], folders: [], comment };
}

function source(comment: string): SourceMeta {
  return { id: 'source-1', created_at: '2026-01-01T00:00:00Z', comment, md5_hash: '' };
}

let mounted: { app: App; host: HTMLElement } | null = null;

interface MountProps {
  book: Book;
  currentSource?: SourceMeta | null;
  readOnly?: boolean;
  onRemoveImportNote?: () => void;
}

function mount(props: MountProps): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);

  const app = createApp({ setup: () => () => h(BookDetail, props) });
  app.mount(host);
  mounted = { app, host };
  return host;
}

function description(host: HTMLElement): HTMLElement | null {
  return host.querySelector('.detail-card-notes .description-body');
}

function removeButtons(host: HTMLElement): HTMLButtonElement[] {
  return Array.from(host.querySelectorAll<HTMLButtonElement>('.detail-card-notes .note-remove'));
}

function textRows(host: HTMLElement): HTMLElement[] {
  return Array.from(host.querySelectorAll<HTMLElement>('.detail-card-notes .note-row dd')).filter(
    (dd) => !dd.querySelector('.description-body')
  );
}

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('BookDetail notes', () => {
  it('renders the description as Markdown and as the HTML an import carried over', () => {
    const host = mount({
      book: book('<p>EPUB 的段落</p>\n\n**粗體**\n\n- 項目一\n- 項目二\n\n> 引文')
    });

    const rendered = description(host);
    expect(rendered).not.toBeNull();
    expect(rendered?.querySelectorAll('p').length).toBeGreaterThan(0);
    expect(rendered?.querySelector('strong')?.textContent).toBe('粗體');
    expect(Array.from(rendered?.querySelectorAll('li') ?? [], (li) => li.textContent))
      .toEqual(['項目一', '項目二']);
    expect(rendered?.querySelector('blockquote')?.textContent?.trim()).toBe('引文');
    expect(rendered?.textContent).not.toContain('<p>');
  });

  it('puts the rendered description inside the row that describes it', () => {
    const host = mount({ book: book('簡介') });

    expect(description(host)?.parentElement?.tagName).toBe('DD');
    expect(host.querySelector('.detail-card-notes .note-row dt')?.textContent?.trim())
      .not.toBe('');
  });

  it('drops active content from the description without dropping its words', () => {
    const host = mount({
      book: book(
        '前言<script>alert(1)</script>' +
          '<img src="x" onerror="alert(1)">' +
          '<a href="javascript:alert(1)">連結文字</a>' +
          '<iframe src="https://example.com"></iframe>' +
          '<style>p { color: red }</style>結尾'
      )
    });

    const rendered = description(host);
    expect(rendered?.querySelector('script, iframe, style, img, a')).toBeNull();
    expect(rendered?.querySelector('[onerror]')).toBeNull();
    expect(rendered?.textContent).toContain('前言');
    expect(rendered?.textContent).toContain('連結文字');
    expect(rendered?.textContent).toContain('結尾');
    expect(rendered?.textContent).not.toContain('alert(1)');
  });

  it('leaves an import note as the literal text it is', () => {
    const host = mount({ book: book(''), currentSource: source('<p>轉換時捨棄了圖片</p>') });

    expect(description(host)).toBeNull();
    const [importNotes] = textRows(host);
    expect(importNotes?.querySelector('p')).toBeNull();
    expect(importNotes?.textContent).toBe('<p>轉換時捨棄了圖片</p>');
  });

  it('leaves out a description that amounts to no words', () => {
    for (const comment of ['', '   ', '<br>', '<p> </p><div></div>', '<img src="x">', '<svg><text>Hello</text></svg>']) {
      const host = mount({ book: book(comment) });
      expect(host.querySelector('.detail-card-notes'), comment).toBeNull();
      mounted?.app.unmount();
      mounted?.host.remove();
      mounted = null;
    }
  });

  it('keeps the notes card when only the import note has something to say', () => {
    const host = mount({ book: book('<br>'), currentSource: source('轉換備註') });

    expect(description(host)).toBeNull();
    expect(textRows(host).map((dd) => dd.textContent)).toEqual(['轉換備註']);
  });
});

describe('BookDetail import note removal', () => {
  it('offers removal on the import note only, never on the description', () => {
    const host = mount({ book: book('書籍備註'), currentSource: source('轉換備註') });

    expect(host.querySelectorAll('.detail-card-notes .note-row')).toHaveLength(2);
    const [button] = removeButtons(host);
    expect(button).toBeDefined();
    expect(button?.closest('.note-row')?.querySelector('dd')?.textContent).toBe('轉換備註');
  });

  it('reports the removal request to the parent, which owns the write', () => {
    const onRemoveImportNote = vi.fn();
    const host = mount({ book: book(''), currentSource: source('轉換備註'), onRemoveImportNote });

    removeButtons(host)[0]?.click();
    expect(onRemoveImportNote).toHaveBeenCalledTimes(1);
  });

  it('hides removal on a read-only shelf, where the note still shows', () => {
    const host = mount({ book: book(''), currentSource: source('轉換備註'), readOnly: true });

    expect(removeButtons(host)).toHaveLength(0);
    expect(textRows(host).map((dd) => dd.textContent)).toEqual(['轉換備註']);
  });
});


describe('BookDetail source facts', () => {
  function facts(host: HTMLElement) {
    return Array.from(host.querySelectorAll('.detail-definition-row'))
      .map(row => [row.querySelector('dt')?.textContent, row.querySelector('dd')?.textContent]);
  }

  it('shows listed pages including missing pages and size for images without text counts', () => {
    const host = mount({ book: book(''), currentSource: {
      ...source(''), format: 'images', char_count: 0, line_count: 2, size_bytes: 2048,
      page_list: { pages: [{ name: 'a.png', missing: false }, { name: 'b.png', missing: true }], unlisted: ['stray.png'] }
    }});
    expect(facts(host)).toEqual([['Pages', '2'], ['Size', '2.0 KB']]);
  });

  it('keeps text counts and adds the source size', () => {
    const host = mount({ book: book(''), currentSource: {
      ...source(''), format: 'txt', char_count: 123, line_count: 2, size_bytes: 512
    }});
    expect(facts(host)).toEqual([['Lines', '2'], ['Characters', '123'], ['Size', '512 B']]);
  });

  it('hides unavailable size and pages on a read-only shelf', () => {
    const host = mount({ book: { ...book(''), format: 'images' }, readOnly: true,
      currentSource: { ...source(''), char_count: 0, line_count: 2 }
    });
    expect(facts(host)).toEqual([]);
  });

  it('shows a known empty size', () => {
    const host = mount({ book: book(''), currentSource: { ...source(''), format: 'txt', size_bytes: 0 } });
    expect(facts(host)).toEqual([['Size', '0 B']]);
  });
});
