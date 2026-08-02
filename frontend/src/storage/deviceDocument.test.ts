import { beforeEach, describe, expect, it, vi } from 'vitest';

const { readFileMock, writeFileMock } = vi.hoisted(() => ({
  readFileMock: vi.fn(),
  writeFileMock: vi.fn()
}));

vi.mock('@capacitor/filesystem', () => ({
  Directory: { Data: 'DATA' },
  Encoding: { UTF8: 'utf8' },
  Filesystem: { readFile: readFileMock, writeFile: writeFileMock }
}));

const {
  buildDeviceDocumentKey,
  createDesktopDocumentStorage,
  createFilesystemDocumentStorage,
  createInMemoryDocumentStorage,
  DeviceDocumentStore
} = await import('./deviceDocument');

const PATH = 'plainshelf-cache/doc.json';

describe('createFilesystemDocumentStorage', () => {
  beforeEach(() => {
    readFileMock.mockReset();
    writeFileMock.mockReset().mockResolvedValue(undefined);
  });

  it('reads and writes the app-private document', async () => {
    const stored = '{"version":1}';
    readFileMock.mockResolvedValue({ data: stored });

    const storage = createFilesystemDocumentStorage(PATH);

    expect(await storage.load()).toBe(stored);
    expect(readFileMock).toHaveBeenCalledWith(
      expect.objectContaining({ path: PATH, directory: 'DATA' })
    );

    await storage.save(stored);
    expect(writeFileMock).toHaveBeenCalledWith(
      expect.objectContaining({ path: PATH, data: stored, recursive: true })
    );
  });

  it('treats a missing file as nothing stored yet', async () => {
    readFileMock.mockRejectedValue(new Error('File does not exist.'));

    expect(await createFilesystemDocumentStorage(PATH).load()).toBeNull();
  });

  // Reporting a failed read as "nothing stored" would let the next write
  // replace a document that was never read, wiping every shelf's data.
  it('propagates a read failure that is not a missing file', async () => {
    readFileMock.mockRejectedValue(new Error('EACCES: permission denied'));

    await expect(createFilesystemDocumentStorage(PATH).load()).rejects.toThrow('permission denied');
  });
});

describe('createDesktopDocumentStorage', () => {
  it('maps the binding’s empty string to nothing stored yet', async () => {
    const storage = createDesktopDocumentStorage({
      read: async () => '',
      write: async () => undefined
    });

    expect(await storage.load()).toBeNull();
  });

  it('passes the document through to the write binding', async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    const storage = createDesktopDocumentStorage({ read: async () => '{"a":1}', write });

    expect(await storage.load()).toBe('{"a":1}');
    await storage.save('{"a":2}');
    expect(write).toHaveBeenCalledWith('{"a":2}');
  });
});

interface CounterDocument {
  count: number;
}

class CounterStore extends DeviceDocumentStore<CounterDocument> {
  bump(by: number): Promise<void> {
    return this.mutate((doc) => ({ count: doc.count + by }));
  }

  noop(): Promise<void> {
    return this.mutate((doc) => doc);
  }

  current(): Promise<CounterDocument> {
    return this.read();
  }
}

function newCounterStore(initial: string | null = null) {
  const storage = createInMemoryDocumentStorage(initial);
  const save = vi.spyOn(storage, 'save');
  const parse = (text: string | null): CounterDocument => ({
    count: text ? (JSON.parse(text) as CounterDocument).count : 0
  });
  return { store: new CounterStore(storage, parse, JSON.stringify), save };
}

describe('DeviceDocumentStore', () => {
  it('serializes concurrent mutations instead of losing one', async () => {
    const { store } = newCounterStore();

    await Promise.all([store.bump(1), store.bump(1), store.bump(1)]);

    expect((await store.current()).count).toBe(3);
  });

  it('does not write when the mutation returns the document unchanged', async () => {
    const { store, save } = newCounterStore();

    await store.noop();

    expect(save).not.toHaveBeenCalled();
  });

  it('keeps accepting mutations after one fails', async () => {
    const storage = createInMemoryDocumentStorage();
    vi.spyOn(storage, 'save').mockRejectedValueOnce(new Error('disk full'));
    const parse = (text: string | null): CounterDocument => ({
      count: text ? (JSON.parse(text) as CounterDocument).count : 0
    });
    const store = new CounterStore(storage, parse, JSON.stringify);

    await expect(store.bump(1)).rejects.toThrow('disk full');
    await store.bump(5);

    expect((await store.current()).count).toBe(5);
  });
});

describe('buildDeviceDocumentKey', () => {
  it('keeps the bare shelf id for a same-origin client', () => {
    expect(buildDeviceDocumentKey('', 'default_shelf')).toBe('default_shelf');
    expect(buildDeviceDocumentKey('   ', 'default_shelf')).toBe('default_shelf');
  });

  // Two servers commonly both call a shelf `default_shelf` while their book ids
  // mean different things, so a repointable client must not share a key.
  it('qualifies the shelf id with the API base on a repointable client', () => {
    expect(buildDeviceDocumentKey('http://host:20000', 'default_shelf')).toBe(
      'http://host:20000|default_shelf'
    );
  });
});
