import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const runtime = vi.hoisted(() => ({ preview: false, reader: false }));

vi.mock('@/api/client', () => ({
  getActiveShelfID: () => 'shelf-1',
  getApiBase: () => '',
  isMockApiMode: () => false
}));

vi.mock('@/providers/runtime', () => ({
  isWailsRuntime: () => true,
  isDesktopShellPreview: () => runtime.preview,
  isDesktopHost: () => !runtime.preview && !runtime.reader,
  isReaderRuntime: () => runtime.reader
}));

vi.mock('@/providers/shell', () => ({ getShell: () => undefined }));

let localStore: Map<string, string>;

function stubWails(app: Record<string, unknown>): void {
  localStore = new Map();
  vi.stubGlobal('window', {
    go: { main: { [runtime.reader ? 'ReaderApp' : 'DesktopApp']: app } },
    localStorage: {
      getItem: (key: string) => localStore.get(key) ?? null,
      setItem: (key: string, value: string) => void localStore.set(key, value)
    }
  });
}

// Sequential: Vitest 5.0.5 resolveMocks() clears its queue before resolving,
// so a concurrent first import can load the real module instead of the mock.
async function loadStorage() {
  const history = await import('./readHistory');
  const stats = await import('./readingStats');
  const progress = await import('./readingProgress');
  return { ...history, ...stats, ...progress };
}

beforeEach(() => {
  vi.resetModules();
  runtime.preview = false;
  runtime.reader = false;
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('Wails runtime without device storage bindings', () => {
  it('rejects instead of falling back to localStorage', async () => {
    stubWails({});
    const storage = await loadStorage();

    await expect(storage.addReadHistory('book-1')).rejects.toThrow('binding not available');
    await expect(storage.getReadHistoryIDs()).rejects.toThrow('binding not available');
    await expect(storage.addReadingSeconds('book-1', 30, '2026-11-01')).rejects.toThrow(
      'binding not available'
    );
    await expect(storage.getReadingActivityRange('2026-11-01', '2026-11-01')).rejects.toThrow(
      'binding not available'
    );
    await expect(
      storage.saveLocalReadingProgress('book-1', { char_offset: 10, at: 1 })
    ).rejects.toThrow('binding not available');
    expect(localStore.size).toBe(0);
  });

  it('rejects when only the removed whole-document bindings exist', async () => {
    stubWails({
      ReadReadHistory: async () => '',
      WriteReadHistory: async () => undefined,
      ReadReadingStats: async () => '',
      WriteReadingStats: async () => undefined
    });
    const storage = await loadStorage();

    await expect(storage.addReadHistory('book-1')).rejects.toThrow('binding not available');
    await expect(storage.addReadingSeconds('book-1', 30, '2026-11-01')).rejects.toThrow(
      'binding not available'
    );
    expect(localStore.size).toBe(0);
  });

  it('keeps the desktop shell preview on browser storage', async () => {
    runtime.preview = true;
    stubWails({});
    const storage = await loadStorage();

    await storage.addReadHistory('book-1');

    expect(await storage.getReadHistoryIDs()).toEqual(['book-1']);
    expect(localStore.size).toBe(1);
  });
});

describe('Wails runtime with desktop bindings', () => {
  it('writes read history through the operation bindings', async () => {
    const app = {
      ReadReadHistory: vi
        .fn()
        .mockResolvedValue('{"version":1,"limit":7,"shelves":{"shelf-1":["book-2","book-1"]}}'),
      RecordReadHistory: vi.fn().mockResolvedValue(undefined),
      ClearReadHistory: vi.fn().mockResolvedValue(undefined),
      SetReadHistoryLimit: vi.fn().mockResolvedValue(undefined)
    };
    stubWails(app);
    const storage = await loadStorage();

    await storage.addReadHistory('book-1');
    await storage.clearReadHistory();
    await storage.setReadHistoryLimit(5);

    expect(app.RecordReadHistory).toHaveBeenCalledWith('shelf-1', 'book-1');
    expect(app.ClearReadHistory).toHaveBeenCalledWith('shelf-1');
    expect(app.SetReadHistoryLimit).toHaveBeenCalledWith(5);
    expect(await storage.getReadHistoryIDs()).toEqual(['book-2', 'book-1']);
    expect(await storage.getReadHistoryLimit()).toBe(7);
    expect(localStore.size).toBe(0);
  });

  it('sends read history writes in call order, even after a failure', async () => {
    const calls: string[] = [];
    let finishRecord: () => void = () => undefined;
    const app = {
      ReadReadHistory: vi.fn().mockResolvedValue(''),
      RecordReadHistory: vi.fn(
        (_key: string, bookID: string) =>
          new Promise<void>((resolve, reject) => {
            calls.push(`record ${bookID}`);
            if (bookID === 'bad') {
              reject(new Error('disk full'));
              return;
            }
            finishRecord = resolve;
          })
      ),
      ClearReadHistory: vi.fn(async () => void calls.push('clear')),
      SetReadHistoryLimit: vi.fn(async () => void calls.push('limit'))
    };
    stubWails(app);
    const storage = await loadStorage();

    const record = storage.addReadHistory('book-1');
    const clear = storage.clearReadHistory();
    await Promise.resolve();
    expect(calls).toEqual(['record book-1']);

    finishRecord();
    await Promise.all([record, clear]);
    await expect(storage.addReadHistory('bad')).rejects.toThrow('disk full');
    await storage.setReadHistoryLimit(5);

    expect(calls).toEqual(['record book-1', 'clear', 'record bad', 'limit']);
  });

  it('adds reading seconds through the operation binding', async () => {
    const app = {
      ReadReadingStats: vi
        .fn()
        .mockResolvedValue('{"version":1,"shelves":{"shelf-1":{"2026-10-31":40}}}'),
      AddReadingSeconds: vi.fn().mockResolvedValue(undefined)
    };
    stubWails(app);
    const storage = await loadStorage();

    await storage.addReadingSeconds('book-1', 30, '2026-11-01');

    expect(app.AddReadingSeconds).toHaveBeenCalledWith('shelf-1', '2026-11-01', 30);
    expect(await storage.getReadingActivityRange('2026-10-01', '2026-11-01')).toEqual({
      '2026-10-31': 40
    });
    expect(localStore.size).toBe(0);
  });
});

describe('Wails runtime with reader bindings', () => {
  it('writes read history and reading stats through the reader app', async () => {
    runtime.reader = true;
    const app = {
      ReadReadHistory: vi
        .fn()
        .mockResolvedValue('{"version":1,"limit":100,"shelves":{"shelf-1":["book-9"]}}'),
      RecordReadHistory: vi.fn().mockResolvedValue(undefined),
      ClearReadHistory: vi.fn().mockResolvedValue(undefined),
      SetReadHistoryLimit: vi.fn().mockResolvedValue(undefined),
      ReadReadingStats: vi
        .fn()
        .mockResolvedValue('{"version":1,"shelves":{"shelf-1":{"2026-10-31":40}}}'),
      AddReadingSeconds: vi.fn().mockResolvedValue(undefined)
    };
    stubWails(app);
    const storage = await loadStorage();

    await storage.addReadHistory('book-1');
    await storage.addReadingSeconds('book-1', 30, '2026-11-01');

    expect(app.RecordReadHistory).toHaveBeenCalledWith('shelf-1', 'book-1');
    expect(app.AddReadingSeconds).toHaveBeenCalledWith('shelf-1', '2026-11-01', 30);
    expect(await storage.getReadHistoryIDs()).toEqual(['book-9']);
    expect(await storage.getReadingActivityRange('2026-10-01', '2026-11-01')).toEqual({
      '2026-10-31': 40
    });
    expect(localStore.size).toBe(0);
  });
});
