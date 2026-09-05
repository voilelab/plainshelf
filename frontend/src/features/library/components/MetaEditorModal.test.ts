// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, reactive, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Book, BookUpdateRequest } from '@/types/book';

const providerMocks = vi.hoisted(() => ({
  getBook: vi.fn(),
  updateBook: vi.fn()
}));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ getBook: providerMocks.getBook }),
  bookshelfWriter: () => ({ updateBook: providerMocks.updateBook })
}));

vi.mock('@/components/BaseDialog.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return {
    default: defineComponent({
      name: 'BaseDialogStub',
      props: {
        open: Boolean,
        title: String,
        busy: Boolean,
        describedBy: String
      },
      emits: ['close'],
      setup(props, { emit, slots }) {
        return () => props.open
          ? h('div', {
              class: 'base-dialog-stub',
              'data-busy': String(props.busy),
              'data-described-by': props.describedBy
            }, [
              h('button', { class: 'base-dialog-escape', onClick: () => emit('close') }, 'escape'),
              h('button', { class: 'base-dialog-backdrop', onClick: () => emit('close') }, 'backdrop'),
              slots.default?.()
            ])
          : null;
      }
    })
  };
});

vi.mock('@/components/ConfirmModal.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return {
    default: defineComponent({
      name: 'ConfirmModalStub',
      props: { open: Boolean },
      emits: ['cancel', 'confirm'],
      setup(props, { emit }) {
        return () => props.open
          ? h('div', { class: 'discard-confirmation' }, [
              h('button', { class: 'confirm-keep', onClick: () => emit('cancel') }, 'keep'),
              h('button', { class: 'confirm-discard', onClick: () => emit('confirm') }, 'discard')
            ])
          : null;
      }
    })
  };
});

vi.mock('./EditBook.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return {
    default: defineComponent({
      name: 'EditBookStub',
      props: {
        book: { type: Object, required: true },
        saving: Boolean,
        error: String,
        embedded: Boolean
      },
      emits: ['submit', 'cancel', 'dirty-change'],
      setup(props, { emit }) {
        // Carries nsfw as well as a plain field: the modal must forward whatever
        // the editor submits rather than a set of fields it knows about, or a
        // control added to the editor would silently stop saving.
        const payload: BookUpdateRequest = { title: 'Updated title', nsfw: true };
        return () => h('div', {
          class: 'edit-book-stub',
          'data-title': (props.book as Book).title,
          'data-saving': String(props.saving),
          'data-error': props.error ?? '',
          'data-embedded': String(props.embedded)
        }, [
          h('button', { class: 'make-dirty', onClick: () => emit('dirty-change', true) }, 'dirty'),
          h('button', { class: 'restore-draft', onClick: () => emit('dirty-change', false) }, 'restore'),
          h('button', { class: 'edit-cancel', onClick: () => emit('cancel') }, 'cancel'),
          h('button', { class: 'edit-submit', onClick: () => emit('submit', payload) }, 'submit')
        ]);
      }
    })
  };
});

import MetaEditorModal from './MetaEditorModal.vue';

interface ModalProps {
  open: boolean;
  bookId: string | null;
}

interface MountedModal {
  app: App;
  host: HTMLElement;
  props: ModalProps;
  closed: number[];
  submitted: BookUpdateRequest[];
  saved: Book[];
  dirtyChanges: boolean[];
}

const mounted: MountedModal[] = [];

function book(id: string, title = `Book ${id}`): Book {
  return { id, title, authors: [], tags: [], folders: [] };
}

function mountModal(initial: Partial<ModalProps> = {}): MountedModal {
  const host = document.createElement('div');
  document.body.append(host);
  const props = reactive<ModalProps>({ open: true, bookId: 'book-1', ...initial });
  const closed: number[] = [];
  const submitted: BookUpdateRequest[] = [];
  const saved: Book[] = [];
  const dirtyChanges: boolean[] = [];

  const app = createApp(defineComponent({
    setup: () => () => h(MetaEditorModal, {
      open: props.open,
      bookId: props.bookId,
      onClose: () => closed.push(1),
      onSubmit: (payload: BookUpdateRequest) => submitted.push(payload),
      onSaved: (value: Book) => saved.push(value),
      onDirtyChange: (value: boolean) => dirtyChanges.push(value)
    })
  }));
  app.mount(host);

  const entry = { app, host, props, closed, submitted, saved, dirtyChanges };
  mounted.push(entry);
  return entry;
}

async function flushAsync(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

function click(host: HTMLElement, selector: string): void {
  const element = host.querySelector<HTMLButtonElement>(selector);
  if (!element) throw new Error(`Missing ${selector}`);
  element.click();
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

beforeEach(() => {
  providerMocks.getBook.mockReset();
  providerMocks.updateBook.mockReset();
});

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

describe('MetaEditorModal loading lifecycle', () => {
  it('fetches the latest metadata every time it opens', async () => {
    providerMocks.getBook
      .mockResolvedValueOnce(book('book-1', 'First read'))
      .mockResolvedValueOnce(book('book-1', 'Changed on disk'));
    const { host, props } = mountModal();

    await flushAsync();
    expect(providerMocks.getBook).toHaveBeenCalledTimes(1);
    expect(host.querySelector('.edit-book-stub')?.getAttribute('data-title')).toBe('First read');

    props.open = false;
    await nextTick();
    props.open = true;
    await flushAsync();

    expect(providerMocks.getBook).toHaveBeenCalledTimes(2);
    expect(host.querySelector('.edit-book-stub')?.getAttribute('data-title')).toBe('Changed on disk');
    expect(host.querySelector('.edit-book-stub')?.getAttribute('data-embedded')).toBe('true');
  });

  it('does not let a stale request replace the newly selected book', async () => {
    const stale = deferred<Book>();
    providerMocks.getBook
      .mockReturnValueOnce(stale.promise)
      .mockResolvedValueOnce(book('book-2', 'Current book'));
    const { host, props } = mountModal();

    props.bookId = 'book-2';
    await flushAsync();
    stale.resolve(book('book-1', 'Stale book'));
    await flushAsync();

    expect(host.querySelector('.edit-book-stub')?.getAttribute('data-title')).toBe('Current book');
  });

  it('shows load errors and retries without closing the modal', async () => {
    providerMocks.getBook
      .mockRejectedValueOnce(new Error('disk read failed'))
      .mockResolvedValueOnce(book('book-1'));
    const { host } = mountModal();

    await flushAsync();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('disk read failed');

    click(host, '.metadata-editor-load-error .button');
    await flushAsync();
    expect(providerMocks.getBook).toHaveBeenCalledTimes(2);
    expect(host.querySelector('.edit-book-stub')).not.toBeNull();
  });
});

describe('MetaEditorModal closing lifecycle', () => {
  it('reports dirty state to the host page and clears it after discarding', async () => {
    providerMocks.getBook.mockResolvedValue(book('book-1'));
    const { host, dirtyChanges } = mountModal();
    await flushAsync();

    click(host, '.make-dirty');
    expect(dirtyChanges.at(-1)).toBe(true);

    click(host, '.metadata-editor-close');
    await nextTick();
    click(host, '.confirm-discard');
    expect(dirtyChanges.at(-1)).toBe(false);
  });

  it('closes immediately when clean from the close button', async () => {
    providerMocks.getBook.mockResolvedValue(book('book-1'));
    const { host, closed } = mountModal();
    await flushAsync();

    click(host, '.metadata-editor-close');
    expect(closed).toHaveLength(1);
  });

  it('protects dirty drafts for cancel, Escape, and backdrop dismissal', async () => {
    providerMocks.getBook.mockResolvedValue(book('book-1'));
    const { host, closed } = mountModal();
    await flushAsync();
    click(host, '.make-dirty');

    for (const selector of ['.edit-cancel', '.base-dialog-escape', '.base-dialog-backdrop']) {
      click(host, selector);
      await nextTick();
      expect(host.querySelector('.discard-confirmation')).not.toBeNull();
      expect(closed).toHaveLength(0);
      click(host, '.confirm-keep');
      await nextTick();
    }

    click(host, '.base-dialog-backdrop');
    await nextTick();
    click(host, '.confirm-discard');
    expect(closed).toHaveLength(1);
  });
});

describe('MetaEditorModal saving lifecycle', () => {
  it('blocks duplicate submit and every close path while saving, then preserves a failed draft', async () => {
    const update = deferred<Book>();
    providerMocks.getBook.mockResolvedValue(book('book-1'));
    providerMocks.updateBook.mockReturnValue(update.promise);
    const { host, closed } = mountModal();
    await flushAsync();
    click(host, '.make-dirty');

    click(host, '.edit-submit');
    click(host, '.edit-submit');
    await nextTick();
    expect(providerMocks.updateBook).toHaveBeenCalledTimes(1);
    expect(host.querySelector('.base-dialog-stub')?.getAttribute('data-busy')).toBe('true');
    expect(host.querySelector<HTMLButtonElement>('.metadata-editor-close')?.disabled).toBe(true);

    click(host, '.base-dialog-escape');
    click(host, '.base-dialog-backdrop');
    expect(closed).toHaveLength(0);
    expect(host.querySelector('.discard-confirmation')).toBeNull();

    update.reject(new Error('write failed'));
    await flushAsync();
    expect(host.querySelector('.edit-book-stub')?.getAttribute('data-error')).toBe('write failed');

    click(host, '.base-dialog-escape');
    await nextTick();
    expect(host.querySelector('.discard-confirmation')).not.toBeNull();
  });

  it('emits the payload and updated book, then closes after a successful save', async () => {
    const updated = book('book-1', 'Updated title');
    providerMocks.getBook.mockResolvedValue(book('book-1'));
    providerMocks.updateBook.mockResolvedValue(updated);
    const { host, submitted, saved, closed } = mountModal();
    await flushAsync();

    click(host, '.edit-submit');
    await flushAsync();

    expect(providerMocks.updateBook).toHaveBeenCalledWith('book-1', { title: 'Updated title', nsfw: true });
    expect(submitted).toEqual([{ title: 'Updated title', nsfw: true }]);
    expect(saved).toEqual([updated]);
    expect(closed).toHaveLength(1);
  });
});
