// @vitest-environment jsdom
import { nextTick } from 'vue';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { SHELF_BACKDROP_MAX_IMAGE_BYTES } from '@/composables/useShelfBackdrop';
import ShelfBackdropModal from './ShelfBackdropModal.vue';

const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

function radio(label: string): HTMLButtonElement {
  const found = document.body.querySelector<HTMLButtonElement>(`[role="radio"][aria-label="${label}"]`);
  if (!found) throw new Error(`no radio labelled ${label}`);
  return found;
}

function button(text: string): HTMLButtonElement {
  const found = [...document.body.querySelectorAll('button')].find((b) => b.textContent?.trim() === text);
  if (!found) throw new Error(`no button ${text}`);
  return found;
}

async function chooseFile(file: File): Promise<void> {
  const input = document.body.querySelector<HTMLInputElement>('[data-testid="backdrop-file"]');
  if (!input) throw new Error('no file input');
  Object.defineProperty(input, 'files', { configurable: true, value: [file] });
  input.dispatchEvent(new Event('change'));
  await settle();
}

async function open(): Promise<void> {
  mount(ShelfBackdropModal, { props: { open: true } });
  await nextTick();
  await settle();
}

beforeEach(() => {
  setLocale('en');
  useShelvesStore().selectedShelfID.value = `shelf-${Math.random()}`;
  URL.createObjectURL = vi.fn(() => 'blob:backdrop');
  URL.revokeObjectURL = vi.fn();
});

describe('ShelfBackdropModal', () => {
  it('marks the picked swatch and enables reset', async () => {
    await open();
    expect(button('Reset to default').disabled).toBe(true);

    radio('Sky').click();
    await settle();

    expect(radio('Sky').getAttribute('aria-checked')).toBe('true');
    expect(radio('Rose').getAttribute('aria-checked')).toBe('false');
    expect(button('Reset to default').disabled).toBe(false);

    button('Reset to default').click();
    await settle();
    expect(radio('Sky').getAttribute('aria-checked')).toBe('false');
  });

  it('shows the size limit error for an oversized image', async () => {
    await open();

    await chooseFile(new File([new Uint8Array(SHELF_BACKDROP_MAX_IMAGE_BYTES + 1)], 'big.png', { type: 'image/png' }));

    expect(document.body.querySelector('[role="alert"]')?.textContent).toContain('larger than 8 MB');
    expect(button('Reset to default').disabled).toBe(true);
  });
});
