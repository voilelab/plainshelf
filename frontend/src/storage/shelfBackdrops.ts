export const SHELF_BACKDROP_SWATCHES = ['sage', 'sky', 'lavender', 'rose', 'sand', 'slate'] as const;

export type ShelfBackdropSwatch = (typeof SHELF_BACKDROP_SWATCHES)[number];

export type ShelfBackdrop = { kind: 'color'; swatch: ShelfBackdropSwatch } | { kind: 'image'; image: Blob };

interface ShelfBackdropStorage {
  get(shelfID: string): Promise<ShelfBackdrop | null>;
  set(shelfID: string, backdrop: ShelfBackdrop): Promise<void>;
  delete(shelfID: string): Promise<void>;
}

function parseShelfBackdrop(value: unknown): ShelfBackdrop | null {
  if (typeof value !== 'object' || value === null) return null;
  const record = value as Record<string, unknown>;
  if (record.kind === 'color' && (SHELF_BACKDROP_SWATCHES as readonly unknown[]).includes(record.swatch)) {
    return { kind: 'color', swatch: record.swatch as ShelfBackdropSwatch };
  }
  if (record.kind === 'image' && record.image instanceof Blob) {
    return { kind: 'image', image: record.image };
  }
  return null;
}

function createInMemoryShelfBackdropStorage(): ShelfBackdropStorage {
  const values = new Map<string, ShelfBackdrop>();
  return {
    get: async (shelfID) => values.get(shelfID) ?? null,
    set: async (shelfID, backdrop) => {
      values.set(shelfID, backdrop);
    },
    delete: async (shelfID) => {
      values.delete(shelfID);
    }
  };
}

const DB_NAME = 'plainshelf-shelf-backdrops';
const STORE = 'backdrops';

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

function transactionDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error ?? new DOMException('Transaction aborted', 'AbortError'));
  });
}

function createIndexedDBShelfBackdropStorage(factory: IDBFactory): ShelfBackdropStorage {
  let opening: Promise<IDBDatabase> | null = null;

  function open(): Promise<IDBDatabase> {
    opening ??= new Promise<IDBDatabase>((resolve, reject) => {
      const request = factory.open(DB_NAME, 1);
      request.onupgradeneeded = () => request.result.createObjectStore(STORE);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    }).catch((err: unknown) => {
      opening = null;
      throw err;
    });
    return opening;
  }

  async function write(apply: (store: IDBObjectStore) => void): Promise<void> {
    const tx = (await open()).transaction(STORE, 'readwrite');
    apply(tx.objectStore(STORE));
    await transactionDone(tx);
  }

  return {
    async get(shelfID) {
      const tx = (await open()).transaction(STORE, 'readonly');
      return parseShelfBackdrop(await requestResult(tx.objectStore(STORE).get(shelfID)));
    },
    set: (shelfID, backdrop) => write((store) => store.put(backdrop, shelfID)),
    delete: (shelfID) => write((store) => store.delete(shelfID))
  };
}

export function createShelfBackdropStorage(): ShelfBackdropStorage {
  return typeof indexedDB === 'undefined'
    ? createInMemoryShelfBackdropStorage()
    : createIndexedDBShelfBackdropStorage(indexedDB);
}
