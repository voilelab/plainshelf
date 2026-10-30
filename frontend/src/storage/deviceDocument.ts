export interface DeviceDocumentStorage {
  load(): Promise<string | null>;
  save(text: string): Promise<void>;
}

export function createLocalStorageDocumentStorage(key: string): DeviceDocumentStorage {
  return {
    async load(): Promise<string | null> {
      if (typeof window === 'undefined') {
        return null;
      }
      return window.localStorage.getItem(key);
    },
    async save(text: string): Promise<void> {
      if (typeof window === 'undefined') {
        return;
      }
      window.localStorage.setItem(key, text);
    }
  };
}

export function createDesktopDocumentStorage(bindings: {
  read: () => Promise<string>;
  write: (text: string) => Promise<void>;
}): DeviceDocumentStorage {
  return {
    async load(): Promise<string | null> {
      const text = await bindings.read();
      return text === '' ? null : text;
    },
    async save(text: string): Promise<void> {
      await bindings.write(text);
    }
  };
}

export function createInMemoryDocumentStorage(
  initial: string | null = null
): DeviceDocumentStorage {
  let stored = initial;
  return {
    async load(): Promise<string | null> {
      return stored;
    },
    async save(text: string): Promise<void> {
      stored = text;
    }
  };
}

export class DeviceDocumentStore<T> {
  private queue: Promise<unknown> = Promise.resolve();

  constructor(
    private readonly storage: DeviceDocumentStorage,
    private readonly parse: (text: string | null) => T,
    private readonly serialize: (doc: T) => string
  ) {}

  protected async read(): Promise<T> {
    return this.parse(await this.storage.load());
  }

  protected mutate(apply: (doc: T) => T): Promise<void> {
    const next = this.queue.then(async () => {
      const current = await this.read();
      const updated = apply(current);
      if (updated !== current) {
        await this.storage.save(this.serialize(updated));
      }
    });
    this.queue = next.catch(() => undefined);
    return next;
  }
}

export function buildDeviceDocumentKey(apiBase: string, shelfID: string): string {
  const base = apiBase.trim();
  return base ? `${base}|${shelfID}` : shelfID;
}
