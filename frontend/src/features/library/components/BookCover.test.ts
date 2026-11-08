// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/providers', () => ({
  bookshelfWriter: () => ({ uploadBookCover: vi.fn(), deleteBookCover: vi.fn() })
}));

vi.mock('@/components/BookCoverImg.vue', () => ({
  default: defineComponent({ name: 'BookCoverImgStub', setup: () => () => h('img') })
}));

vi.mock('./GenerateCoverModal.vue', () => ({
  default: defineComponent({ name: 'GenerateCoverModalStub', setup: () => () => null })
}));

vi.mock('@/components/ConfirmModal.vue', () => ({
  default: defineComponent({ name: 'ConfirmModalStub', setup: () => () => null })
}));

import BookCover from './BookCover.vue';

const mounted: { app: App; host: HTMLElement }[] = [];

function mount(): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({
    setup: () => () => h(BookCover, { bookId: 'book-1', title: '書名', authors: [] })
  });
  app.mount(host);
  mounted.push({ app, host });
  return host;
}

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
});

function tray(host: HTMLElement): HTMLElement | null {
  return host.querySelector<HTMLElement>('.cover-action-tray');
}

function toggle(host: HTMLElement): HTMLButtonElement {
  const button = host.querySelector<HTMLButtonElement>('.cover-options-toggle');
  if (!button) throw new Error('the cover-options toggle is missing');
  return button;
}

describe('BookCover options tray', () => {
  it('keeps the action buttons mounted while the tray is collapsed', () => {
    const host = mount();

    const collapsed = tray(host);
    expect(collapsed).not.toBeNull();
    expect(collapsed!.hasAttribute('hidden')).toBe(true);
    expect(collapsed!.querySelectorAll('.cover-btn')).toHaveLength(3);
  });

  it('reveals the same tray when the toggle is opened', async () => {
    const host = mount();

    toggle(host).click();
    await new Promise((resolve) => setTimeout(resolve));
    await nextTick();

    const opened = tray(host);
    expect(opened!.hasAttribute('hidden')).toBe(false);
    expect(opened!.querySelectorAll('.cover-btn')).toHaveLength(3);
    expect(toggle(host).getAttribute('aria-expanded')).toBe('true');
  });
});

function fileDragEvent(type: string): Event & { dataTransfer: { dropEffect: string } } {
  const event = new Event(type, { bubbles: true, cancelable: true });
  const dataTransfer = { types: ['Files'], items: [{ kind: 'file' }], files: [], dropEffect: 'none' };
  Object.defineProperty(event, 'dataTransfer', { value: dataTransfer });
  return event as Event & { dataTransfer: { dropEffect: string } };
}

describe('BookCover file drag', () => {
  it.each(['dragenter', 'dragover'])('keeps copy on %s past a root listener', (type) => {
    const host = mount();
    const rejectAtRoot = (event: Event): void => {
      (event as DragEvent).dataTransfer!.dropEffect = 'none';
    };
    document.documentElement.addEventListener(type, rejectAtRoot);

    try {
      const event = fileDragEvent(type);
      host.querySelector('.cover-drop-target')!.dispatchEvent(event);

      expect(event.dataTransfer.dropEffect, 'a root listener must not cancel the cover drop').toBe('copy');
      expect(event.defaultPrevented).toBe(true);
    } finally {
      document.documentElement.removeEventListener(type, rejectAtRoot);
    }
  });
});
