/**
 * @vitest-environment node
 */
import 'fake-indexeddb/auto';
import { describe, expect, it } from 'vitest';
import { createShelfBackdropStorage } from './shelfBackdrops';

function putRaw(shelfID: string, value: unknown): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('plainshelf-shelf-backdrops', 1);
    request.onsuccess = () => {
      const tx = request.result.transaction('backdrops', 'readwrite');
      tx.objectStore('backdrops').put(value, shelfID);
      tx.oncomplete = () => {
        request.result.close();
        resolve();
      };
      tx.onerror = () => reject(tx.error);
    };
    request.onerror = () => reject(request.error);
  });
}

describe('shelf backdrop storage', () => {
  it('keeps a color and an image per shelf id across reopening', async () => {
    const image = new Blob([new Uint8Array([1, 2, 3])], { type: 'image/png' });
    const first = createShelfBackdropStorage();
    await first.set('keep-a', { kind: 'color', swatch: 'sky' });
    await first.set('keep-b', { kind: 'image', image });

    const reopened = createShelfBackdropStorage();
    expect(await reopened.get('keep-a')).toEqual({ kind: 'color', swatch: 'sky' });
    const stored = await reopened.get('keep-b');
    expect(stored?.kind).toBe('image');
    if (stored?.kind !== 'image') return;
    expect(stored.image.type).toBe('image/png');
    expect([...new Uint8Array(await stored.image.arrayBuffer())]).toEqual([1, 2, 3]);
    expect(await reopened.get('keep-c')).toBeNull();
  });

  it('removes only the cleared shelf', async () => {
    const storage = createShelfBackdropStorage();
    await storage.set('clear-a', { kind: 'color', swatch: 'rose' });
    await storage.set('clear-b', { kind: 'color', swatch: 'sand' });

    await storage.delete('clear-a');

    expect(await storage.get('clear-a')).toBeNull();
    expect(await storage.get('clear-b')).toEqual({ kind: 'color', swatch: 'sand' });
  });

  it('reads an unknown swatch or a non-blob image as no backdrop', async () => {
    const storage = createShelfBackdropStorage();
    await storage.get('warm-up');
    await putRaw('bad-color', { kind: 'color', swatch: '#ff0000' });
    await putRaw('bad-image', { kind: 'image', image: 'data:image/png;base64,' });

    expect(await storage.get('bad-color')).toBeNull();
    expect(await storage.get('bad-image')).toBeNull();
  });
});
