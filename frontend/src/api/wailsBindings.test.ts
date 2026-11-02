import { afterEach, describe, expect, it, vi } from 'vitest';
import { callWails, hasWailsMethod, wailsHost } from './wailsBindings';

interface Probe {
  Echo: (text: string) => Promise<string>;
}

afterEach(() => {
  vi.unstubAllGlobals();
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
