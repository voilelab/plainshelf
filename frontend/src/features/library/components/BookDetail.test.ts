// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

// The breadcrumb renders RouterLink and so needs a router; the notes card is
// what this file is about.
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

/** The rendered description, which is the only notes row carrying markup. */
function description(host: HTMLElement): HTMLElement | null {
  return host.querySelector('.detail-card-notes .description-body');
}

/** The import note's remove control, which the description row never gets. */
function removeButtons(host: HTMLElement): HTMLButtonElement[] {
  return Array.from(host.querySelectorAll<HTMLButtonElement>('.detail-card-notes .note-remove'));
}

/** The notes rows that stay literal text, in the order the card lists them. */
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
    // The tags are structure now, not the characters the page used to print.
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

    // Two rows, one button: the description is the user's own text and is
    // edited through the metadata editor, not removed from here.
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

describe('BookDetail newer-schema notice', () => {
  /** The notice, which sits above the heading rather than in a card. */
  function notice(host: HTMLElement): HTMLElement | null {
    return host.querySelector('.schema-notice');
  }

  it('tells the reader the book was saved in a newer format', () => {
    const host = mount({ book: { ...book(''), schema_newer_than_supported: true } });

    const shown = notice(host);
    expect(shown).not.toBeNull();
    expect(shown?.getAttribute('role')).toBe('status');
    // Whichever locale the test runs in, the notice names the app rather than
    // calling the book broken.
    expect(shown?.textContent).toContain('PlainShelf');
  });

  it('shows nothing for a book whose book.json carried no such mark', () => {
    expect(notice(mount({ book: book('') }))).toBeNull();
  });

  it('shows nothing when the mark is explicitly false', () => {
    expect(notice(mount({ book: { ...book(''), schema_newer_than_supported: false } }))).toBeNull();
  });
});
