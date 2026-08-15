import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { ReadFileOptions, WriteFileOptions, DeleteFileOptions } from '@capacitor/filesystem';

// Minimal in-memory stand-in for the plugin, shared between the hoisted
// vi.mock factory and the tests. Only the three operations this store uses are
// modelled; see filesystemMobileBookCache.test.ts for the fuller model.
const fsModel = vi.hoisted(() => {
  const store = new Map<string, string>();
  const normalize = (directory: string | undefined, path: string): string => {
    const clean = path.replace(/^\/+|\/+$/g, '');
    return directory ? `${directory}/${clean}` : clean;
  };
  return { store, normalize };
});

vi.mock('@capacitor/filesystem', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@capacitor/filesystem')>();
  const { store, normalize } = fsModel;

  return {
    ...actual,
    Filesystem: {
      async writeFile(options: WriteFileOptions) {
        const key = normalize(options.directory, options.path);
        if (typeof options.data !== 'string') {
          throw new Error('Mock filesystem only supports string data.');
        }
        store.set(key, options.data);
        return { uri: key };
      },
      async readFile(options: ReadFileOptions) {
        const key = normalize(options.directory, options.path);
        const data = store.get(key);
        if (data === undefined) {
          throw new Error('File does not exist.');
        }
        return { data };
      },
      async deleteFile(options: DeleteFileOptions) {
        const key = normalize(options.directory, options.path);
        if (!store.delete(key)) {
          throw new Error('File does not exist.');
        }
      }
    }
  };
});

// Imported after vi.mock so the implementation binds to the mocked plugin.
import { setActiveShelfID, setApiBase } from '@/api/client';
import { buildDeviceDocumentKey } from '@/storage/deviceDocument';
import {
  FilesystemShelfSnapshotStore,
  InMemoryShelfSnapshotStore,
  SHELF_SNAPSHOT_VERSION,
  parseShelfSnapshot
} from './shelfSnapshotStore';
import type { PersistedShelfSnapshot } from './shelfSnapshotStore';

const HOST_A = 'pcloud://api.pcloud.com';
const SHELF_A = '/PlainShelf/default-shelf';
const SHELF_B = '/PlainShelf/comics';

function connectTo(apiBase: string, shelfID: string): void {
  setApiBase(apiBase);
  setActiveShelfID(shelfID);
}

function snapshotPathFor(apiBase: string, shelfID: string): string {
  const scope = encodeURIComponent(buildDeviceDocumentKey(apiBase, shelfID));
  return `DATA/plainshelf-cache/scopes/${scope}/shelf-snapshot.json`;
}

function makeSnapshot(overrides: Partial<PersistedShelfSnapshot> = {}): PersistedShelfSnapshot {
  return {
    version: SHELF_SNAPSHOT_VERSION,
    shelf_root: SHELF_A,
    fetched_at: 1_700_000_000_000,
    layers: ['/', 'Fiction'],
    books: [
      {
        pkg: {
          folderName: 'a.bookpkg',
          folderid: 10,
          layers: ['Fiction'],
          meta: { fileid: 11, name: 'book.json', size: 42, modified: 'Sun, 16 Mar 2014 17:26:04 +0000' },
          files: {},
          sources: []
        },
        meta: { id: 'a', title: 'A' }
      }
    ],
    ...overrides
  };
}

beforeEach(() => {
  fsModel.store.clear();
  connectTo(HOST_A, SHELF_A);
});

describe('parseShelfSnapshot', () => {
  it('accepts a snapshot this version wrote', () => {
    expect(parseShelfSnapshot(makeSnapshot())).not.toBeNull();
  });

  it.each([
    ['a different version', { version: SHELF_SNAPSHOT_VERSION + 1 }],
    ['a missing timestamp', { fetched_at: undefined as unknown as number }],
    ['layers of the wrong type', { layers: 'Fiction' as unknown as string[] }]
  ])('rejects %s', (_label, overrides) => {
    expect(parseShelfSnapshot(makeSnapshot(overrides))).toBeNull();
  });

  it('rejects a book with no id, rather than listing a nameless entry', () => {
    const snapshot = makeSnapshot();
    snapshot.books[0].meta = { title: 'A' } as never;

    expect(parseShelfSnapshot(snapshot)).toBeNull();
  });

  it.each([[null], [[]], ['{}'], [42]])('rejects the non-object %p', (value) => {
    expect(parseShelfSnapshot(value)).toBeNull();
  });
});

describe('InMemoryShelfSnapshotStore', () => {
  it('round-trips a snapshot and clears it', async () => {
    const store = new InMemoryShelfSnapshotStore();
    expect(await store.load()).toBeNull();

    await store.save(makeSnapshot());
    expect(await store.load()).toEqual(makeSnapshot());

    await store.clear();
    expect(await store.load()).toBeNull();
  });

  it('hands out copies, so a caller cannot mutate the stored snapshot', async () => {
    const store = new InMemoryShelfSnapshotStore();
    await store.save(makeSnapshot());

    const loaded = await store.load();
    loaded!.books.length = 0;

    expect((await store.load())!.books).toHaveLength(1);
  });
});

describe('FilesystemShelfSnapshotStore', () => {
  it('round-trips a snapshot through the device', async () => {
    const store = new FilesystemShelfSnapshotStore();

    await store.save(makeSnapshot());

    expect(fsModel.store.has(snapshotPathFor(HOST_A, SHELF_A))).toBe(true);
    expect(await store.load()).toEqual(makeSnapshot());
  });

  it('returns null when nothing has been saved', async () => {
    expect(await new FilesystemShelfSnapshotStore().load()).toBeNull();
  });

  it('keeps two shelves apart', async () => {
    const store = new FilesystemShelfSnapshotStore();
    await store.save(makeSnapshot());

    connectTo(HOST_A, SHELF_B);
    expect(await store.load()).toBeNull();

    await store.save(makeSnapshot({ shelf_root: SHELF_B, books: [] }));
    connectTo(HOST_A, SHELF_A);
    expect((await store.load())!.shelf_root).toBe(SHELF_A);
  });

  it('treats malformed JSON as nothing cached', async () => {
    fsModel.store.set(snapshotPathFor(HOST_A, SHELF_A), '{not json');

    expect(await new FilesystemShelfSnapshotStore().load()).toBeNull();
  });

  it('treats a snapshot from a future version as nothing cached', async () => {
    fsModel.store.set(
      snapshotPathFor(HOST_A, SHELF_A),
      JSON.stringify(makeSnapshot({ version: SHELF_SNAPSHOT_VERSION + 1 }))
    );

    expect(await new FilesystemShelfSnapshotStore().load()).toBeNull();
  });

  it('reports a failed write as a warning, not as a failed shelf update', async () => {
    const { Filesystem } = await import('@capacitor/filesystem');
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    vi.spyOn(Filesystem, 'writeFile').mockRejectedValueOnce(new Error('No space left on device'));

    await expect(new FilesystemShelfSnapshotStore().save(makeSnapshot())).resolves.toBeUndefined();
    expect(warn).toHaveBeenCalled();

    vi.restoreAllMocks();
  });

  it('clears a saved snapshot, and clearing an absent one is not an error', async () => {
    const store = new FilesystemShelfSnapshotStore();
    await store.save(makeSnapshot());

    await store.clear();
    expect(await store.load()).toBeNull();

    await expect(store.clear()).resolves.toBeUndefined();
  });
});
