import { afterEach, describe, expect, it, vi } from 'vitest';
import { callWails, hasWailsMethod, wailsHost } from './wailsBindings';

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

describe('wailsHost', () => {
  it('reads the desktop marker', () => {
    stubWindow({ __PLAINSHELF_DESKTOP__: true });
    expect(wailsHost()).toBe('DesktopApp');
  });

  it('reads the reader marker', () => {
    stubWindow({ __PLAINSHELF_READER__: {} });
    expect(wailsHost()).toBe('ReaderApp');
  });

  it('ignores window.go without a marker', () => {
    stubWindow({ go: { main: { DesktopApp: {} } } });
    expect(wailsHost()).toBeNull();
  });
});

describe('callWails', () => {
  it('calls the bound method', async () => {
    const echo = vi.fn(async (text: string) => text);
    stubWindow({ __PLAINSHELF_DESKTOP__: true, go: { main: { DesktopApp: { Echo: echo } } } });

    expect(await callWails<Probe, 'Echo'>('DesktopApp', 'Echo', 'hi')).toBe('hi');
    expect(echo).toHaveBeenCalledWith('hi');
  });

  it('throws with the method name when Wails lacks it', () => {
    stubWindow({ __PLAINSHELF_DESKTOP__: true, go: { main: { DesktopApp: {} } } });

    expect(() => callWails<Probe, 'Echo'>('DesktopApp', 'Echo', 'hi')).toThrow(
      'Wails binding DesktopApp.Echo is missing'
    );
    expect(hasWailsMethod('DesktopApp', 'Echo')).toBe(false);
  });

  it('throws outside Wails even when window.go has the method', () => {
    const echo = vi.fn(async (text: string) => text);
    stubWindow({ go: { main: { DesktopApp: { Echo: echo } } } });

    expect(() => callWails<Probe, 'Echo'>('DesktopApp', 'Echo', 'hi')).toThrow(
      'DesktopApp.Echo called outside'
    );
    expect(hasWailsMethod('DesktopApp', 'Echo')).toBe(false);
    expect(echo).not.toHaveBeenCalled();
  });

  it('refuses the other app in the reader window', () => {
    stubWindow({
      __PLAINSHELF_READER__: {},
      go: { main: { DesktopApp: { Echo: async (text: string) => text } } }
    });

    expect(() => callWails<Probe, 'Echo'>('DesktopApp', 'Echo', 'hi')).toThrow('called outside');
  });
});

describe('callWails on Wails v3', () => {
  it('calls the method by its bound name', async () => {
    v3.byName.mockResolvedValue('hi');
    stubWindow({ __PLAINSHELF_READER__: {} });

    expect(await callWails<Probe, 'Echo'>('ReaderApp', 'Echo', 'hi')).toBe('hi');
    expect(v3.byName).toHaveBeenCalledWith('main.ReaderApp.Echo', 'hi');
    expect(hasWailsMethod('ReaderApp', 'Echo')).toBe(true);
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
    expect(hasWailsMethod('ReaderApp', 'Echo')).toBe(false);
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
  it('skips the v3 runtime on v2 and outside Wails', async () => {
    const loaded = vi.fn();
    const startWailsRuntime = await freshStart(loaded);
    stubWindow({ __PLAINSHELF_DESKTOP__: true, go: { main: { DesktopApp: {} } } });
    startWailsRuntime();
    stubWindow({});
    startWailsRuntime();
    await vi.dynamicImportSettled();

    expect(loaded).not.toHaveBeenCalled();
  });

  it('loads the v3 runtime in a window without window.go', async () => {
    const loaded = vi.fn();
    const startWailsRuntime = await freshStart(loaded);
    stubWindow({ __PLAINSHELF_READER__: {} });
    startWailsRuntime();

    await vi.waitFor(() => expect(loaded).toHaveBeenCalledOnce());
  });
});
