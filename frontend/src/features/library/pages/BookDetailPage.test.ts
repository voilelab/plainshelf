// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, ref, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';

const mocks = vi.hoisted(() => ({
  route: { params: { id: 'book-1' }, query: {} as Record<string, string> },
  routerPush: vi.fn(),
  fetchDetail: vi.fn(),
  dismissActionError: vi.fn(),
  ensureShelvesLoaded: vi.fn(),
  writesEnabled: null as any,
  detailBook: null as any
}));

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.routerPush })
}));

vi.mock('reka-ui', async () => {
  const { defineComponent, h } = await import('vue');
  const passthrough = (name: string, tag = 'div') => defineComponent({
    name,
    setup(_, { slots }) {
      return () => h(tag, slots.default?.());
    }
  });
  return {
    DropdownMenuRoot: passthrough('DropdownMenuRoot'),
    DropdownMenuTrigger: passthrough('DropdownMenuTrigger', 'button'),
    DropdownMenuPortal: passthrough('DropdownMenuPortal'),
    DropdownMenuContent: passthrough('DropdownMenuContent'),
    DropdownMenuSeparator: passthrough('DropdownMenuSeparator'),
    DropdownMenuItem: defineComponent({
      name: 'DropdownMenuItem',
      emits: ['select'],
      setup(_, { emit, slots }) {
        return () => h('button', { type: 'button', onClick: () => emit('select') }, slots.default?.());
      }
    })
  };
});

vi.mock('@/features/library/composables/useBookDetail', async () => {
  const { ref } = await import('vue');
  mocks.detailBook = ref<Book | null>(null);
  return {
    useBookDetail: () => ({
      book: mocks.detailBook,
      progress: ref({ char_offset: 25, percent: 25 }),
      progressContentLength: ref(null),
      currentSource: ref(null),
      chapters: ref([{ index: 0, title: 'One' }]),
      loading: ref(false),
      error: ref(''),
      fetchDetail: mocks.fetchDetail
    })
  };
});

vi.mock('@/composables/useWriteAccess', async () => {
  const { ref } = await import('vue');
  mocks.writesEnabled = ref(true);
  return { useWriteAccess: () => ({ writesEnabled: mocks.writesEnabled }) };
});

vi.mock('@/composables/useBookActions', async () => {
  const { ref } = await import('vue');
  const empty = () => {};
  return {
    useBookActions: () => ({
      downloading: ref(false),
      actionError: ref(''),
      deleteTarget: ref(null),
      deleting: ref(false),
      moveTarget: ref(null),
      moving: ref(false),
      moveFolderOptions: ref([]),
      copyTarget: ref(null),
      copying: ref(false),
      copyFolderOptions: ref([]),
      transferTarget: ref(null),
      transferMode: ref(null),
      transferStatus: ref(''),
      transferPercentage: ref(0),
      transferError: ref(''),
      transferStarted: ref(false),
      transferring: ref(false),
      transferFinished: ref(false),
      requestTransfer: empty,
      cancelTransfer: empty,
      submitTransfer: empty,
      canOpenBookFolder: ref(false),
      goRead: empty,
      openBookFolder: empty,
      downloadBook: empty,
      requestMove: empty,
      cancelMove: empty,
      submitMove: empty,
      requestCopy: empty,
      cancelCopy: empty,
      submitCopy: empty,
      requestDelete: empty,
      cancelDelete: empty,
      confirmDelete: empty,
      dismissActionError: mocks.dismissActionError
    })
  };
});

vi.mock('@/composables/useShelvesStore', async () => {
  const { ref } = await import('vue');
  return {
    useShelvesStore: () => ({
      shelves: ref([]),
      selectedShelfID: ref('shelf-1'),
      ensureShelvesLoaded: mocks.ensureShelvesLoaded
    })
  };
});

vi.mock('@/composables/useOfflineDownload', async () => {
  const { ref } = await import('vue');
  return {
    useOfflineDownload: () => ({
      state: ref('not_downloaded'),
      error: ref(''),
      supported: ref(false),
      refresh: vi.fn(),
      download: vi.fn(),
      remove: vi.fn()
    })
  };
});

vi.mock('@/composables/useDocumentTitle', () => ({ useDocumentTitle: vi.fn() }));
vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ saveReadProgress: vi.fn() }),
  bookshelfWriter: () => ({ refreshSourceMeta: vi.fn() })
}));

vi.mock('@/features/library/components/MetaEditorModal.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return {
    default: defineComponent({
      name: 'MetaEditorModalStub',
      props: { open: Boolean, bookId: String },
      emits: ['close', 'saved'],
      setup(props, { emit }) {
        const updated: Book = {
          id: 'book-1',
          title: 'Updated title',
          authors: ['New Author'],
          tags: ['new-tag'],
          language: 'ja',
          published_at: '2026-08-28',
          comment: 'Updated note',
          star: 5,
          identifiers: { isbn: '9780000000000' },
          folders: ['fiction']
        };
        return () => props.open
          ? h('div', { class: 'metadata-modal-stub', 'data-book-id': props.bookId }, [
              h('button', { class: 'metadata-close', onClick: () => emit('close') }, 'close'),
              h('button', {
                class: 'metadata-save',
                onClick: () => {
                  emit('saved', updated);
                  emit('close');
                }
              }, 'save')
            ])
          : null;
      }
    })
  };
});

vi.mock('@/features/library/components/BookDetail.vue', async () => {
  const { defineComponent, h, ref } = await import('vue');
  return {
    default: defineComponent({
      name: 'BookDetailStub',
      props: { book: { type: Object, required: true } },
      setup(props, { slots }) {
        const expanded = ref(false);
        return () => h('div', { class: 'book-detail-stub' }, [
          h('output', { class: 'book-json' }, JSON.stringify(props.book)),
          h('button', { class: 'expand-chapters', onClick: () => { expanded.value = true; } }, 'expand'),
          h('span', { class: 'chapter-state' }, expanded.value ? 'expanded' : 'collapsed'),
          slots.reading?.()
        ]);
      }
    })
  };
});

vi.mock('@/features/library/components/BookCover.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return { default: defineComponent({ setup: () => () => h('div') }) };
});
vi.mock('@/components/DeleteModal.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return { default: defineComponent({ setup: () => () => h('div') }) };
});
vi.mock('@/features/library/components/MoveBooksModal.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return { default: defineComponent({ setup: () => () => h('div') }) };
});
vi.mock('@/features/library/components/TransferBookModal.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return { default: defineComponent({ setup: () => () => h('div') }) };
});
vi.mock('@/components/ProgressBar.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return { default: defineComponent({ setup: () => () => h('div') }) };
});

import { setLocale } from '@/i18n';
import BookDetailPage from './BookDetailPage.vue';

let mounted: { app: App; host: HTMLElement } | null = null;

function originalBook(): Book {
  return {
    id: 'book-1',
    title: 'Original title',
    authors: ['Old Author'],
    tags: ['old-tag'],
    language: 'en',
    comment: 'Old note',
    star: 1,
    identifiers: { isbn: 'old' },
    folders: ['fiction']
  };
}

function mountPage(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => h(BookDetailPage) }));
  app.mount(host);
  mounted = { app, host };
  return host;
}

async function flush(): Promise<void> {
  await Promise.resolve();
  await nextTick();
}

function editMetadataButton(host: HTMLElement): HTMLButtonElement | undefined {
  return [...host.querySelectorAll<HTMLButtonElement>('.reka-menu-item')]
    .find((button) => button.textContent?.includes('Edit book details'));
}

beforeEach(() => {
  setLocale('en');
  mocks.route.params.id = 'book-1';
  mocks.route.query = {};
  mocks.routerPush.mockReset();
  mocks.fetchDetail.mockReset().mockResolvedValue(undefined);
  mocks.dismissActionError.mockReset();
  mocks.ensureShelvesLoaded.mockReset();
  mocks.writesEnabled.value = true;
  mocks.detailBook.value = originalBook();
});

afterEach(() => {
  mounted?.app.unmount();
  mounted?.host.remove();
  mounted = null;
});

describe('BookDetailPage metadata editor', () => {
  it('opens and cancels the modal without changing the route or adding history', async () => {
    const host = mountPage();
    await flush();

    editMetadataButton(host)?.click();
    await flush();
    expect(host.querySelector('.metadata-modal-stub')?.getAttribute('data-book-id')).toBe('book-1');

    host.querySelector<HTMLButtonElement>('.metadata-close')?.click();
    await flush();
    expect(host.querySelector('.metadata-modal-stub')).toBeNull();
    expect(mocks.routerPush).not.toHaveBeenCalled();
  });

  it('applies every returned metadata field immediately while preserving detail state', async () => {
    const host = mountPage();
    await flush();
    host.querySelector<HTMLButtonElement>('.expand-chapters')?.click();
    editMetadataButton(host)?.click();
    await flush();

    host.querySelector<HTMLButtonElement>('.metadata-save')?.click();
    await flush();

    const rendered = host.querySelector('.book-json')?.textContent ?? '';
    for (const value of [
      'Updated title', 'New Author', 'new-tag', 'ja', '2026-08-28',
      'Updated note', '5', '9780000000000'
    ]) {
      expect(rendered).toContain(value);
    }
    expect(host.querySelector('.chapter-state')?.textContent).toBe('expanded');
    expect(host.textContent).toContain('Book details saved.');
    expect(mocks.fetchDetail).toHaveBeenCalledTimes(1);
    expect(mocks.routerPush).not.toHaveBeenCalled();
  });

  it('does not render or open the editor on a read-only provider', async () => {
    mocks.writesEnabled.value = false;
    const host = mountPage();
    await flush();

    expect(editMetadataButton(host)).toBeUndefined();
    expect(host.querySelector('.metadata-modal-stub')).toBeNull();
    expect(mocks.routerPush).not.toHaveBeenCalled();
  });
});
