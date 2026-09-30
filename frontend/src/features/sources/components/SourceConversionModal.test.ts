// @vitest-environment jsdom
import { createApp, h, nextTick, reactive, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import SourceConversionModal from './SourceConversionModal.vue';
import { setLocale } from '@/i18n';

// Reka's dialog measures the scrollbar and traps focus; jsdom has neither
// ResizeObserver nor a layout, and the modal's own behavior does not depend on
// either.
class NoopResizeObserver {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
globalThis.ResizeObserver ??= NoopResizeObserver as unknown as typeof ResizeObserver;

let app: App | null = null;

// The dialog content is teleported to <body>, so the fields are looked up there
// rather than under the mount host.
function mount(content: string) {
  const state = reactive({ open: true });
  const host = document.createElement('div');
  document.body.append(host);

  app = createApp({
    setup: () => () =>
      h(SourceConversionModal, {
        open: state.open,
        kind: 'line-count-md',
        sourceId: 'src-1',
        content,
        onCancel: () => {
          state.open = false;
        }
      })
  });
  app.mount(host);
  return { host };
}

function field(): HTMLInputElement {
  const input = document.body.querySelector<HTMLInputElement>('input[role="spinbutton"]');
  if (!input) throw new Error('the line-count field is missing');
  return input;
}

function commit(input: HTMLInputElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('blur'));
}

function previewText(): string {
  return document.body.querySelector('.conversion-preview')?.textContent ?? '';
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  app?.unmount();
  app = null;
  document.body.innerHTML = '';
});

const CONTENT = Array.from({ length: 12 }, (_, index) => `line ${index + 1}`).join('\n');

describe('SourceConversionModal line-count field', () => {
  it('opens on the default chapter size and previews the split', async () => {
    mount(CONTENT);
    await nextTick();

    expect(field().value).toBe('1000');
    // Formatted without a thousands separator, so the box reads as a number the
    // conversion actually uses.
    expect(previewText()).not.toContain('1,000');
  });

  it('previews the split for a typed size', async () => {
    mount(CONTENT);
    await nextTick();

    commit(field(), '5');
    await nextTick();

    // 12 lines in chapters of five is three chapters.
    expect(previewText()).toContain('3');
  });

  it('clamps a size below one instead of converting with it', async () => {
    mount(CONTENT);
    await nextTick();

    commit(field(), '0');
    await nextTick();

    expect(field().value).toBe('1');
    expect(document.body.querySelector('.conversion-error')).toBeNull();
  });

  it('reports an emptied box as an invalid size rather than converting', async () => {
    mount(CONTENT);
    await nextTick();

    commit(field(), '');
    await nextTick();

    const error = document.body.querySelector('.conversion-error');
    expect(error?.textContent).toBeTruthy();
    // The confirm button is the modal's own, not one of the field's steppers.
    const confirm = Array.from(document.body.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Create source'
    );
    expect(confirm?.disabled).toBe(true);
  });
});
