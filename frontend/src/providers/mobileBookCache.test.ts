import { beforeEach, describe, expect, it } from 'vitest';

import type { Book } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { InMemoryMobileBookCache, type CachedBookManifest } from './mobileBookCache';

function makeBook(id: string): Book {
  return {
    id,
    title: `Title of ${id}`,
    authors: ['Author A'],
    tags: ['fiction'],
    layers: ['shelf-1']
  };
}

function makeSource(id: string): SourceMeta {
  return {
    id,
    created_at: '2026-07-01T00:00:00Z',
    comment: `comment for ${id}`,
    md5_hash: `md5-${id}`
  };
}

function makeManifest(bookId: string, sourceIds: string[]): CachedBookManifest {
  return {
    book: makeBook(bookId),
    sources: sourceIds.map(makeSource),
    downloaded_at: '2026-07-10T12:00:00Z'
  };
}

describe('InMemoryMobileBookCache', () => {
  let cache: InMemoryMobileBookCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
  });

  // Source contents are keyed by book and source id joined together, and
  // removeDownloadedBook drops a book's entries by matching that key's prefix.
  // The prefix has to include the separator, or removing `book1` would take
  // `book10`'s sources with it.
  it('removes only the named book, not one whose id it prefixes', async () => {
    await cache.saveDownloadedBook(makeManifest('book1', ['s1']));
    await cache.saveDownloadedBook(makeManifest('book10', ['s1']));
    await cache.saveCachedSourceContent('book1', 's1', 'text of book1');
    await cache.saveCachedSourceContent('book10', 's1', 'text of book10');

    await cache.removeDownloadedBook('book1');

    expect(await cache.getCachedSourceContent('book1', 's1')).toBeNull();
    expect(await cache.getCachedSourceContent('book10', 's1')).toBe('text of book10');
    expect(await cache.getDownloadState('book10')).toBe('downloaded');
  });

  it('keeps source contents apart when one source id prefixes another', async () => {
    await cache.saveCachedSourceContent('book1', 's1', 'first');
    await cache.saveCachedSourceContent('book1', 's10', 'second');

    expect(await cache.getCachedSourceContent('book1', 's1')).toBe('first');
    expect(await cache.getCachedSourceContent('book1', 's10')).toBe('second');
  });

  // Only the manifest's own objects are detached; see cloneManifest for what
  // stays shared below that level.
  it("detaches a stored manifest's own objects from the caller", async () => {
    const manifest = makeManifest('book1', ['s1']);
    await cache.saveDownloadedBook(manifest);
    manifest.book.title = 'mutated after saving';
    manifest.sources[0].id = 'mutated';

    const [stored] = await cache.listDownloadedManifests();
    expect(stored.book.title).toBe('Title of book1');
    expect(stored.sources[0].id).toBe('s1');
  });
});
