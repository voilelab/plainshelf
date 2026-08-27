// @vitest-environment jsdom
import { createApp, h, nextTick, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

// BookDetail is mounted here only to compare its rendered description against
// the preview; its breadcrumb needs a router that neither component is about.
vi.mock('./FolderBreadcrumb.vue', () => ({
  default: { setup: () => () => h('nav') }
}));

import BookDetail from './BookDetail.vue';
import EditBook from './EditBook.vue';
import type { Book, BookUpdateRequest } from '@/types/book';

function book(comment: string): Book {
  return { id: 'book-1', title: '書名', authors: [], tags: [], folders: [], comment };
}

interface Mounted {
  app: App;
  host: HTMLElement;
  submitted: BookUpdateRequest[];
  dirtyChanges: boolean[];
}

const mounted: Mounted[] = [];

function mount(
  comment: string,
  options: { embedded?: boolean; saving?: boolean } = {}
): Mounted {
  const host = document.createElement('div');
  document.body.append(host);
  const submitted: BookUpdateRequest[] = [];
  const dirtyChanges: boolean[] = [];

  const app = createApp({
    setup: () => () =>
      h(EditBook, {
        book: book(comment),
        saving: options.saving ?? false,
        embedded: options.embedded,
        onSubmit: (payload: BookUpdateRequest) => submitted.push(payload),
        onDirtyChange: (dirty: boolean) => dirtyChanges.push(dirty)
      })
  });
  app.mount(host);

  const entry = { app, host, submitted, dirtyChanges };
  mounted.push(entry);
  return entry;
}

function mountDetail(comment: string): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);

  const app = createApp({ setup: () => () => h(BookDetail, { book: book(comment) }) });
  app.mount(host);
  mounted.push({ app, host, submitted: [], dirtyChanges: [] });
  return host;
}

function commentField(host: HTMLElement): HTMLTextAreaElement {
  const textarea = host.querySelector<HTMLTextAreaElement>('.textarea');
  if (!textarea) throw new Error('the comment textarea is missing');
  return textarea;
}

function toggle(host: HTMLElement): HTMLButtonElement {
  const button = host.querySelector<HTMLButtonElement>('.comment-preview-toggle');
  if (!button) throw new Error('the preview toggle is missing');
  return button;
}

/** Types into the comment field the way v-model reads it. */
async function type(host: HTMLElement, value: string): Promise<void> {
  const textarea = commentField(host);
  textarea.value = value;
  textarea.dispatchEvent(new Event('input'));
  await nextTick();
}

async function openPreview(host: HTMLElement): Promise<void> {
  toggle(host).click();
  await nextTick();
}

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

describe('EditBook comment preview', () => {
  it('stays closed until it is asked for, and closes again', async () => {
    const { host } = mount('簡介');

    expect(host.querySelector('.comment-preview')).toBeNull();
    expect(toggle(host).getAttribute('aria-expanded')).toBe('false');

    await openPreview(host);
    expect(host.querySelector('.comment-preview')).not.toBeNull();
    expect(toggle(host).getAttribute('aria-expanded')).toBe('true');
    expect(toggle(host).getAttribute('aria-controls'))
      .toBe(host.querySelector('.comment-preview')?.id);

    await openPreview(host);
    expect(host.querySelector('.comment-preview')).toBeNull();
  });

  it('follows what is typed', async () => {
    const { host } = mount('');
    await openPreview(host);

    expect(host.querySelector('.comment-preview-empty')).not.toBeNull();

    await type(host, '**粗體**');
    expect(host.querySelector('.description-body strong')?.textContent).toBe('粗體');

    await type(host, '- 一\n- 二');
    expect(host.querySelectorAll('.description-body li').length).toBe(2);
  });

  // The point of the preview is that it is not a second renderer. A separate
  // one would drift, and whoever is writing the description believes this one.
  it('produces exactly what the detail page renders from the same source', async () => {
    const source = '<p>EPUB 的段落</p>\n\n**粗體**\n\n- 項目一\n- 項目二\n\n> 引文';
    const { host } = mount(source);
    await openPreview(host);

    const preview = host.querySelector('.comment-preview .description-body');
    const detail = mountDetail(source).querySelector('.detail-card-notes .description-body');

    expect(preview?.innerHTML).toBe(detail?.innerHTML);
    expect(preview?.innerHTML).not.toBe('');
  });

  it('sanitizes the preview as the detail page would', async () => {
    const { host } = mount('前言<script>alert(1)</script><img src="x" onerror="alert(1)">結尾');
    await openPreview(host);

    const preview = host.querySelector('.comment-preview');
    expect(preview?.querySelector('script, img')).toBeNull();
    expect(preview?.querySelector('[onerror]')).toBeNull();
    expect(preview?.textContent).toContain('前言');
    expect(preview?.textContent).toContain('結尾');
    expect(preview?.textContent).not.toContain('alert(1)');
  });

  it('submits the source text, not the rendered preview', async () => {
    const source = '**粗體**與 <em>標籤</em>';
    const { host, submitted } = mount('');
    await openPreview(host);
    await type(host, source);

    host.querySelector<HTMLFormElement>('.edit-form')?.dispatchEvent(
      new Event('submit', { cancelable: true })
    );
    await nextTick();

    expect(submitted).toHaveLength(1);
    expect(submitted[0]?.comment).toBe(source);
  });
});

describe('EditBook dirty state', () => {
  it('starts clean, becomes dirty with input, and returns clean when the draft is restored', async () => {
    const { host, dirtyChanges } = mount('原始內容');
    expect(dirtyChanges).toEqual([false]);

    await type(host, '尚未儲存');
    expect(dirtyChanges.at(-1)).toBe(true);

    await type(host, '原始內容');
    expect(dirtyChanges.at(-1)).toBe(false);
  });
});

describe('EditBook embedded mode', () => {
  it('leaves the modal to provide the heading and locks the draft while saving', () => {
    const { host } = mount('', { embedded: true, saving: true });

    expect(host.querySelector('.edit-header')).toBeNull();
    expect(host.querySelector('.edit-panel-embedded')).not.toBeNull();
    expect(host.querySelector('.edit-panel')?.classList.contains('panel')).toBe(false);
    expect(host.querySelector('.edit-form')?.getAttribute('aria-busy')).toBe('true');
    expect(host.querySelector('.edit-form-fields')?.hasAttribute('inert')).toBe(true);
    expect(Array.from(host.querySelectorAll<HTMLButtonElement>('.form-actions .button'))
      .every((button) => button.disabled)).toBe(true);
  });
});
