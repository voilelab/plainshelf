import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ShelfBackdrop, createShelfBackdropStorage } from '@/storage/shelfBackdrops';

const flushPromises = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

const stored = new Map<string, ShelfBackdrop>();
const failNextSet = { error: null as unknown };

vi.mock('@/storage/shelfBackdrops', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/storage/shelfBackdrops')>();
  const storage: ReturnType<typeof createShelfBackdropStorage> = {
    get: async (id) => stored.get(id) ?? null,
    set: async (id, backdrop) => {
      if (failNextSet.error) {
        const err = failNextSet.error;
        failNextSet.error = null;
        throw err;
      }
      stored.set(id, backdrop);
    },
    delete: async (id) => {
      stored.delete(id);
    }
  };
  return { ...actual, createShelfBackdropStorage: () => storage };
});

async function setup(shelfID = 'shelf-a') {
  vi.resetModules();
  const { useShelvesStore } = await import('@/composables/useShelvesStore');
  const backdrop = await import('./useShelfBackdrop');
  const { selectedShelfID } = useShelvesStore();
  selectedShelfID.value = shelfID;
  const api = backdrop.useShelfBackdrop();
  await flushPromises();
  return { ...api, selectedShelfID, maxBytes: backdrop.SHELF_BACKDROP_MAX_IMAGE_BYTES };
}

function png(size = 4): File {
  return new File([new Uint8Array(size)], 'cover.png', { type: 'image/png' });
}

beforeEach(() => {
  stored.clear();
  failNextSet.error = null;
  let next = 0;
  URL.createObjectURL = vi.fn(() => `blob:backdrop-${++next}`);
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useShelfBackdrop', () => {
  it('adds no class or style for a shelf that never had a backdrop', async () => {
    const { layoutClass, layoutStyle, current } = await setup();

    expect(current.value).toBeNull();
    expect(layoutClass.value).toBe('');
    expect(layoutStyle.value).toEqual({});
  });

  it('stores a swatch under the shelf id and points the layout at its token', async () => {
    const { setColor, layoutClass, layoutStyle } = await setup();

    await setColor('sage');

    expect(stored.get('shelf-a')).toEqual({ kind: 'color', swatch: 'sage' });
    expect(layoutClass.value).toBe('shelf-backdrop-color');
    expect(layoutStyle.value).toEqual({ '--shelf-backdrop-color': 'var(--shelf-backdrop-sage)' });
  });

  it('applies the other shelf backdrop as soon as the shelf changes', async () => {
    stored.set('shelf-b', { kind: 'color', swatch: 'rose' });
    const { setColor, selectedShelfID, layoutStyle, layoutClass } = await setup();
    await setColor('sky');

    selectedShelfID.value = 'shelf-b';
    await flushPromises();
    expect(layoutStyle.value).toEqual({ '--shelf-backdrop-color': 'var(--shelf-backdrop-rose)' });

    selectedShelfID.value = 'shelf-c';
    await flushPromises();
    expect(layoutClass.value).toBe('');
  });

  it('reads the stored backdrop back after a reload', async () => {
    stored.set('shelf-a', { kind: 'color', swatch: 'slate' });

    const { layoutStyle } = await setup();

    expect(layoutStyle.value).toEqual({ '--shelf-backdrop-color': 'var(--shelf-backdrop-slate)' });
  });

  it('shows an image through an object URL and revokes the previous one', async () => {
    const { setImage, setColor, layoutClass, layoutStyle } = await setup();

    await setImage(png());

    expect(stored.get('shelf-a')?.kind).toBe('image');
    expect(layoutClass.value).toBe('shelf-backdrop-image');
    expect(layoutStyle.value).toEqual({ '--shelf-backdrop-image': 'url("blob:backdrop-1")' });

    await setColor('sand');
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:backdrop-1');
  });

  it('rejects an image over the size limit with an explicit error', async () => {
    const { setImage, maxBytes, current } = await setup();

    await expect(setImage(png(maxBytes + 1))).rejects.toThrow('8 MB');

    expect(stored.has('shelf-a')).toBe(false);
    expect(current.value).toBeNull();
  });

  it('rejects a file that is not an image', async () => {
    const { setImage } = await setup();

    await expect(setImage(new File(['x'], 'notes.txt', { type: 'text/plain' }))).rejects.toThrow('not an image');
    expect(stored.has('shelf-a')).toBe(false);
  });

  it('turns a storage quota failure into an explicit error and keeps the old backdrop', async () => {
    const { setImage, setColor, layoutStyle } = await setup();
    await setColor('sky');
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    failNextSet.error = new DOMException('full', 'QuotaExceededError');

    await expect(setImage(png())).rejects.toThrow('no room left');

    expect(stored.get('shelf-a')).toEqual({ kind: 'color', swatch: 'sky' });
    expect(layoutStyle.value).toEqual({ '--shelf-backdrop-color': 'var(--shelf-backdrop-sky)' });
  });

  it('clears back to the default look', async () => {
    const { setColor, clear, layoutClass } = await setup();
    await setColor('lavender');

    await clear();

    expect(stored.has('shelf-a')).toBe(false);
    expect(layoutClass.value).toBe('');
  });
});
