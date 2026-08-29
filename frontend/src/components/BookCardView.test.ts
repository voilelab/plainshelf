// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Book } from '@/types/book';

vi.mock('reka-ui', async () => {
  const { defineComponent, h } = await import('vue');
  const passThrough = (name: string) => defineComponent({
    name,
    setup: (_, { slots }) => () => h('div', slots.default?.())
  });
  return {
    ContextMenuRoot: passThrough('ContextMenuRoot'),
    ContextMenuTrigger: passThrough('ContextMenuTrigger'),
    ContextMenuPortal: passThrough('ContextMenuPortal'),
    ContextMenuContent: passThrough('ContextMenuContent'),
    ContextMenuItem: defineComponent({
      name: 'ContextMenuItem',
      emits: ['select'],
      setup: (_, { emit, slots }) => () => h('button', {
        class: 'context-item',
        onClick: () => emit('select')
      }, slots.default?.())
    })
  };
});

vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({ name: 'BookCoverImgStub', setup: () => () => h('div') })
}));

vi.mock('@/components/BookSelectionCheckbox.vue', () => ({
  default: defineComponent({ name: 'BookSelectionCheckboxStub', setup: () => () => h('div') })
}));

vi.mock('@/composables/useBookItemInteractions', () => ({
  useBookItemInteractions: ({ onActivate }: { onActivate: (payload: unknown) => void }) => ({
    draggingBookId: { value: null },
    onClick: () => onActivate({ id: 'book-1', metaKey: false, ctrlKey: false, shiftKey: false }),
    onPointerDown: vi.fn(),
    onPointerMove: vi.fn(),
    cancelLongPress: vi.fn(),
    onDragStart: vi.fn(),
    onDragEnd: vi.fn()
  })
}));

import BookCardView from './BookCardView.vue';

const mounted: Array<{ app: App; host: HTMLElement }> = [];
const sampleBook: Book = {
  id: 'book-1',
  title: 'One',
  authors: [],
  tags: [],
  folders: []
};

function mountCard(readOnly = false, books: Book[] = [sampleBook]) {
  const host = document.createElement('div');
  document.body.append(host);
  const edits: string[] = [];
  const activations: unknown[] = [];
  const app = createApp(defineComponent({
    setup: () => () => h(BookCardView, {
      books,
      readOnly,
      onEdit: (id: string) => edits.push(id),
      onActivate: (payload: unknown) => activations.push(payload)
    })
  }));
  app.mount(host);
  mounted.push({ app, host });
  return { host, edits, activations };
}

function editButton(host: HTMLElement): HTMLButtonElement | undefined {
  return [...host.querySelectorAll<HTMLButtonElement>('.context-item')]
    .find((button) => button.textContent?.trim() === 'Edit');
}

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

describe('BookCardView metadata edit action', () => {
  it('emits edit without activating the card', async () => {
    const { host, edits, activations } = mountCard();
    editButton(host)?.click();
    await nextTick();

    expect(edits).toEqual(['book-1']);
    expect(activations).toEqual([]);
  });

  it('does not render the edit action in read-only mode', () => {
    const { host } = mountCard(true);
    expect(editButton(host)).toBeUndefined();
  });
});

describe('BookCardView download state', () => {
  it('marks each card with the state it carries', () => {
    const { host } = mountCard(false, [
      { ...sampleBook, id: 'a', download_state: 'not_downloaded' },
      { ...sampleBook, id: 'b', download_state: 'downloaded' },
      { ...sampleBook, id: 'c', download_state: 'update_available' }
    ]);

    const states = [...host.querySelectorAll('.book-download-badge')]
      .map((badge) => badge.getAttribute('data-download-state'));
    expect(states).toEqual(['not_downloaded', 'downloaded', 'update_available']);
  });

  it('leaves cards unmarked when the backend has no download concept', () => {
    const { host } = mountCard();
    expect(host.querySelector('.book-download-badge')).toBeNull();
  });
});
