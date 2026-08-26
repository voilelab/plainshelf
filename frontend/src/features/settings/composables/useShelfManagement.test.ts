// @vitest-environment jsdom
import { nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The composable resolves the shelf-ID preview and the reveal-in-Finder action
// through the active bookshelf provider; hold a swappable provider so each case
// controls what those bindings do (or whether they exist at all).
const provider = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
vi.mock('@/providers', () => ({ getBookshelfProvider: () => provider.value }));

import { useShelfManagement } from './useShelfManagement';
import { setLocale } from '@/i18n';

beforeEach(() => {
  setLocale('en');
  provider.value = {};
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('useShelfManagement shelf-ID preview', () => {
  it('previews the id the backend would assign as the name changes', async () => {
    provider.value = {
      previewDesktopShelfID: vi.fn((name: string) => Promise.resolve(name === '小說' ? 'shelf' : `id-${name}`))
    };
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = '小說';
    await nextTick();
    await vi.waitFor(() => expect(newShelfIDPreview.value).toBe('shelf'));
    expect(provider.value.previewDesktopShelfID).toHaveBeenCalledWith('小說');
  });

  it('clears the preview for an empty name without asking the backend', async () => {
    const previewDesktopShelfID = vi.fn(() => Promise.resolve('unexpected'));
    provider.value = { previewDesktopShelfID };
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = '   ';
    await nextTick();
    await Promise.resolve();
    expect(newShelfIDPreview.value).toBe('');
    expect(previewDesktopShelfID).not.toHaveBeenCalled();
  });

  it('leaves the preview empty when the provider cannot preview', async () => {
    provider.value = {};
    const { newShelfName, newShelfIDPreview } = useShelfManagement();

    newShelfName.value = 'My Books';
    await nextTick();
    await Promise.resolve();
    expect(newShelfIDPreview.value).toBe('');
  });
});

describe('useShelfManagement openShelfFolder', () => {
  it('asks the provider to reveal the shelf folder', async () => {
    const openDesktopShelfFolder = vi.fn(() => Promise.resolve());
    provider.value = { openDesktopShelfFolder };
    const { openShelfFolder, shelfOpError } = useShelfManagement();

    await openShelfFolder('my-books');
    expect(openDesktopShelfFolder).toHaveBeenCalledWith('my-books');
    expect(shelfOpError.value).toBe('');
  });

  it('surfaces a reveal failure on the panel', async () => {
    provider.value = {
      openDesktopShelfFolder: vi.fn(() => Promise.reject(new Error('boom')))
    };
    const { openShelfFolder, shelfOpError } = useShelfManagement();

    await openShelfFolder('my-books');
    expect(shelfOpError.value).toBe('boom');
  });

  it('is a no-op when the provider cannot reveal folders', async () => {
    provider.value = {};
    const { openShelfFolder, shelfOpError } = useShelfManagement();

    await openShelfFolder('my-books');
    expect(shelfOpError.value).toBe('');
  });
});
