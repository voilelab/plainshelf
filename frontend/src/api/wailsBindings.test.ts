import { afterEach, describe, expect, it, vi } from 'vitest';
import { callWails, wailsHost } from './wailsBindings';

const v3 = vi.hoisted(() => ({ byName: vi.fn() }));

vi.mock('@wailsio/runtime', () => ({ Call: { ByName: v3.byName } }));

interface Probe {
  Echo: (text: string) => Promise<string>;
}

afterEach(() => {
  vi.unstubAllGlobals();
  v3.byName.mockReset();
});

function stubWindow(win: Record<string, unknown>): void {
  vi.stubGlobal('window', win);
}

function stubDocument(onListen: (type: string) => void = () => {}): void {
  vi.stubGlobal('document', { documentElement: { addEventListener: onListen } });
}

describe('wailsHost', () => {
  it('reads the desktop marker', () => {
    stubWindow({ __PLAINSHELF_DESKTOP__: true });
    expect(wailsHost()).toBe('DesktopApp');
  });

  it('reads the reader marker', () => {
    stubWindow({ __PLAINSHELF_READER__: {} });
    expect(wailsHost()).toBe('ReaderApp');
  });

  it('is null without a marker', () => {
    stubWindow({});
    expect(wailsHost()).toBeNull();
  });
});

describe('callWails', () => {
  it('calls the method by its bound name', async () => {
    v3.byName.mockResolvedValue('hi');
    stubWindow({ __PLAINSHELF_READER__: {} });

    expect(await callWails<Probe, 'Echo'>('ReaderApp', 'Echo', 'hi')).toBe('hi');
    expect(v3.byName).toHaveBeenCalledWith('main.ReaderApp.Echo', 'hi');
  });

  it('rejects with the method name when Wails lacks it', async () => {
    v3.byName.mockRejectedValue(new ReferenceError('unknown bound method name'));
    stubWindow({ __PLAINSHELF_READER__: {} });

    await expect(callWails<Probe, 'Echo'>('ReaderApp', 'Echo', 'hi')).rejects.toThrow(
      'Wails binding ReaderApp.Echo is missing'
    );
  });

  it('passes a method error through', async () => {
    v3.byName.mockRejectedValue(new Error('disk full'));
    stubWindow({ __PLAINSHELF_READER__: {} });

    await expect(callWails<Probe, 'Echo'>('ReaderApp', 'Echo', 'hi')).rejects.toThrow('disk full');
  });

  it('throws outside the Wails window', () => {
    stubWindow({});

    expect(() => callWails<Probe, 'Echo'>('ReaderApp', 'Echo', 'hi')).toThrow('called outside');
    expect(v3.byName).not.toHaveBeenCalled();
  });

  it('refuses the other app in the reader window', () => {
    stubWindow({ __PLAINSHELF_READER__: {} });

    expect(() => callWails<Probe, 'Echo'>('DesktopApp', 'Echo', 'hi')).toThrow('called outside');
    expect(v3.byName).not.toHaveBeenCalled();
  });
});

async function freshStart(loaded: () => void): Promise<() => void> {
  vi.resetModules();
  vi.doMock('@wailsio/runtime', () => {
    loaded();
    return { Call: { ByName: v3.byName } };
  });
  return (await import('./wailsBindings')).startWailsRuntime;
}

describe('startWailsRuntime', () => {
  it('skips the runtime outside Wails', async () => {
    const loaded = vi.fn();
    const startWailsRuntime = await freshStart(loaded);
    stubWindow({});
    startWailsRuntime();
    await vi.dynamicImportSettled();

    expect(loaded).not.toHaveBeenCalled();
  });

  it('loads the runtime in the desktop window', async () => {
    const loaded = vi.fn();
    const startWailsRuntime = await freshStart(loaded);
    stubDocument();
    stubWindow({ __PLAINSHELF_DESKTOP__: true });
    startWailsRuntime();

    await vi.waitFor(() => expect(loaded).toHaveBeenCalledOnce());
  });

  it('guards file drops before the runtime loads', async () => {
    const order: string[] = [];
    const startWailsRuntime = await freshStart(() => order.push('runtime'));
    stubDocument((type) => order.push(type));
    stubWindow({ __PLAINSHELF_DESKTOP__: true });
    startWailsRuntime();

    await vi.waitFor(() => expect(order).toContain('runtime'));
    expect(order, 'the guard must precede the runtime listeners').toEqual(['dragenter', 'dragover', 'runtime']);
  });

  it('loads the runtime in the reader window', async () => {
    const loaded = vi.fn();
    const startWailsRuntime = await freshStart(loaded);
    stubDocument();
    stubWindow({ __PLAINSHELF_READER__: {} });
    startWailsRuntime();

    await vi.waitFor(() => expect(loaded).toHaveBeenCalledOnce());
  });
});
