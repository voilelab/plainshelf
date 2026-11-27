// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick, type App } from 'vue';

const { getSourceAsset, uploadBookCoverBlob } = vi.hoisted(() => ({
  getSourceAsset: vi.fn(),
  uploadBookCoverBlob: vi.fn()
}));
vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ getSourceAsset }),
  bookshelfWriter: () => ({ uploadBookCoverBlob })
}));

vi.mock('@/components/ConfirmModal.vue', () => ({
  default: defineComponent({
    name: 'ConfirmModalStub',
    props: { open: Boolean, confirmDisabled: Boolean, busy: Boolean },
    emits: ['confirm', 'cancel'],
    setup: (props, { slots, emit }) => () => props.open
      ? h('div', [
          slots.default?.(),
          h('button', { class: 'confirm', disabled: props.confirmDisabled || props.busy, onClick: () => emit('confirm') })
        ])
      : null
  })
}));

import PickPageCoverModal from './PickPageCoverModal.vue';

const mounted: { app: App; host: HTMLElement }[] = [];
const pageImage = new Blob(['png'], { type: 'image/png' });

function mount(onSaved = vi.fn()) {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({
    setup: () => () => h(PickPageCoverModal, {
      open: true,
      bookId: 'comic',
      sourceId: 'pages',
      pages: [
        { name: 'p-a.png', missing: false },
        { name: 'p-b.png', missing: true },
        { name: 'p-c.webp', missing: false }
      ],
      onSaved
    })
  });
  app.mount(host);
  mounted.push({ app, host });
  return { host, onSaved };
}

async function settle(): Promise<void> {
  for (let i = 0; i < 4; i++) await Promise.resolve();
  await nextTick();
}

beforeEach(() => {
  getSourceAsset.mockReset().mockResolvedValue(pageImage);
  uploadBookCoverBlob.mockReset().mockResolvedValue(undefined);
  vi.stubGlobal('URL', { createObjectURL: vi.fn(() => 'blob:page'), revokeObjectURL: vi.fn() });
});

afterEach(() => {
  for (const entry of mounted.splice(0)) {
    entry.app.unmount();
    entry.host.remove();
  }
  vi.unstubAllGlobals();
});

function thumbs(host: HTMLElement): HTMLButtonElement[] {
  return [...host.querySelectorAll<HTMLButtonElement>('.page-cover-thumb')];
}

describe('PickPageCoverModal', () => {
  it('lists every page and disables missing ones', () => {
    const { host } = mount();
    const buttons = thumbs(host);
    expect(buttons.map((button) => button.textContent?.trim())).toEqual(['1', 'Missing2', '3']);
    expect(buttons.map((button) => button.disabled)).toEqual([false, true, false]);
  });

  it('waits for a selection before confirming', async () => {
    const { host } = mount();
    const confirm = host.querySelector<HTMLButtonElement>('.confirm')!;
    expect(confirm.disabled).toBe(true);

    thumbs(host)[2].click();
    await nextTick();
    expect(thumbs(host)[2].getAttribute('aria-pressed')).toBe('true');
    expect(confirm.disabled).toBe(false);
  });

  it('uploads the chosen page image as the cover', async () => {
    const { host, onSaved } = mount();
    thumbs(host)[2].click();
    await nextTick();
    host.querySelector<HTMLButtonElement>('.confirm')!.click();
    await settle();

    expect(getSourceAsset).toHaveBeenCalledWith('comic', 'pages', 'p-c.webp');
    expect(uploadBookCoverBlob).toHaveBeenCalledWith('comic', pageImage);
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  it('shows the upload error and stays open', async () => {
    uploadBookCoverBlob.mockRejectedValue(new Error('read-only shelf'));
    const { host, onSaved } = mount();
    thumbs(host)[0].click();
    await nextTick();
    host.querySelector<HTMLButtonElement>('.confirm')!.click();
    await settle();

    expect(host.querySelector('[role="alert"]')?.textContent).toContain('read-only shelf');
    expect(onSaved).not.toHaveBeenCalled();
  });
});
