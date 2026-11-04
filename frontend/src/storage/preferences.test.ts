/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const runtime = vi.hoisted(() => ({ host: 'browser' as 'browser' | 'desktop' | 'reader' | 'preview' }));

vi.mock('@/providers/runtime', () => ({
  isDesktopHost: () => runtime.host === 'desktop',
  isReaderRuntime: () => runtime.host === 'reader',
  isWailsRuntime: () => runtime.host !== 'browser',
  isDesktopShellPreview: () => runtime.host === 'preview'
}));

type Binding = {
  GetPreferences: ReturnType<typeof vi.fn>;
  SetPreference: ReturnType<typeof vi.fn>;
};

function stubHost(app: 'DesktopApp' | 'ReaderApp', binding: Partial<Binding>): void {
  (window as unknown as { go?: unknown }).go = { main: { [app]: binding } };
}

function hostBinding(initial: Record<string, string>): Binding {
  return {
    GetPreferences: vi.fn(async () => ({ ...initial })),
    SetPreference: vi.fn(async () => undefined)
  };
}

async function loadModule() {
  const preferences = await import('./preferences');
  await preferences.loadPreferences();
  return preferences;
}

beforeEach(() => {
  vi.resetModules();
  window.localStorage.clear();
  runtime.host = 'browser';
});

afterEach(() => {
  delete (window as unknown as { go?: unknown }).go;
  vi.restoreAllMocks();
});

describe('preferences in the browser', () => {
  it('reads and writes localStorage', async () => {
    window.localStorage.setItem('plainshelf.locale', 'zh-Hant');
    const { getPreference, setPreference } = await loadModule();

    expect(getPreference('plainshelf.locale')).toBe('zh-Hant');
    setPreference('app-zoom-level', '1.25');
    expect(window.localStorage.getItem('app-zoom-level')).toBe('1.25');
    setPreference('app-zoom-level', null);
    expect(window.localStorage.getItem('app-zoom-level')).toBeNull();
  });

  it('notifies subscribers in the same document until they unsubscribe', async () => {
    const { setPreference, subscribePreference } = await loadModule();
    const seen: (string | null)[] = [];
    const unsubscribe = subscribePreference('reader-font-size', (value) => seen.push(value));

    setPreference('reader-font-size', '24');
    setPreference('reader-font-size', '24');
    unsubscribe();
    setPreference('reader-font-size', '26');

    expect(seen).toEqual(['24']);
  });

  it('forwards storage events from other tabs', async () => {
    const { subscribePreference } = await loadModule();
    const listener = vi.fn();
    subscribePreference('show-nsfw-device', listener);

    window.dispatchEvent(
      new StorageEvent('storage', {
        key: 'show-nsfw-device',
        newValue: 'true',
        storageArea: window.localStorage
      })
    );

    expect(listener).toHaveBeenCalledWith('true');
  });
});

describe('preferences in the desktop and reader apps', () => {
  it.each([
    ['desktop', 'DesktopApp'],
    ['reader', 'ReaderApp']
  ] as const)('serves the %s snapshot and writes through its binding', async (host, app) => {
    runtime.host = host;
    window.localStorage.setItem('reader-font-size', '14');
    const binding = hostBinding({ 'reader-font-size': '28' });
    stubHost(app, binding);
    const { getPreference, setPreference } = await loadModule();

    expect(getPreference('reader-font-size')).toBe('28');
    setPreference('reader-font-size', '30');
    setPreference('plainshelf.shelf', null);
    expect(getPreference('reader-font-size')).toBe('30');
    await vi.waitFor(() => expect(binding.SetPreference).toHaveBeenCalledTimes(1));
    expect(binding.SetPreference).toHaveBeenCalledWith('reader-font-size', '30');
    expect(window.localStorage.getItem('reader-font-size')).toBe('14');
  });

  it('sends writes in call order, even after a failure', async () => {
    runtime.host = 'desktop';
    const order: string[] = [];
    const binding = hostBinding({});
    binding.SetPreference.mockImplementation(async (key: string, value: string) => {
      if (value === '1') {
        await new Promise((resolve) => setTimeout(resolve, 10));
        order.push(value);
        throw new Error('disk full');
      }
      order.push(value);
    });
    stubHost('DesktopApp', binding);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const { setPreference } = await loadModule();

    setPreference('plainshelf.sidebar.expandedWidth', '1');
    setPreference('plainshelf.sidebar.expandedWidth', '2');
    setPreference('plainshelf.sidebar.expandedWidth', null);

    await vi.waitFor(() => expect(order).toEqual(['1', '2', '']));
  });

  it('starts from defaults when the snapshot cannot be read', async () => {
    runtime.host = 'desktop';
    const binding = hostBinding({});
    binding.GetPreferences.mockRejectedValue(new Error('locked'));
    stubHost('DesktopApp', binding);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const { getPreference, setPreference } = await loadModule();

    expect(getPreference('plainshelf.locale')).toBeNull();
    setPreference('plainshelf.locale', 'en');
    await vi.waitFor(() => expect(binding.SetPreference).toHaveBeenCalledWith('plainshelf.locale', 'en'));
  });

  it('keeps preferences in memory, not localStorage, without the binding', async () => {
    runtime.host = 'desktop';
    stubHost('DesktopApp', {});
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const { getPreference, setPreference } = await loadModule();

    setPreference('app-zoom-level', '2');
    expect(getPreference('app-zoom-level')).toBe('2');
    expect(window.localStorage.getItem('app-zoom-level')).toBeNull();
  });

  it('keeps the desktop shell preview on localStorage', async () => {
    runtime.host = 'preview';
    const { setPreference } = await loadModule();

    setPreference('app-zoom-level', '2');
    expect(window.localStorage.getItem('app-zoom-level')).toBe('2');
  });

  it('reloads values another app wrote when the window regains focus', async () => {
    runtime.host = 'desktop';
    const binding = hostBinding({ 'reader-font-size': '20', 'reader-font-family': 'system' });
    stubHost('DesktopApp', binding);
    const { getPreference, subscribePreference } = await loadModule();
    const listener = vi.fn();
    subscribePreference('reader-font-size', listener);
    binding.GetPreferences.mockResolvedValue({ 'reader-font-size': '28' });

    window.dispatchEvent(new Event('focus'));

    await vi.waitFor(() => expect(listener).toHaveBeenCalledWith('28'));
    expect(getPreference('reader-font-size')).toBe('28');
    expect(getPreference('reader-font-family')).toBeNull();
  });

  it('keeps a local change made while the reload is in flight', async () => {
    runtime.host = 'desktop';
    const binding = hostBinding({ 'app-zoom-level': '1' });
    stubHost('DesktopApp', binding);
    const { getPreference, setPreference } = await loadModule();
    let finishRead!: (values: Record<string, string>) => void;
    binding.GetPreferences.mockReturnValue(new Promise((resolve) => (finishRead = resolve)));

    window.dispatchEvent(new Event('focus'));
    await vi.waitFor(() => expect(binding.GetPreferences.mock.calls.length).toBeGreaterThan(1));
    setPreference('app-zoom-level', '1.5');
    finishRead({ 'app-zoom-level': '1' });
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(getPreference('app-zoom-level')).toBe('1.5');
  });

  it('keeps the snapshot when a reload fails', async () => {
    runtime.host = 'desktop';
    const binding = hostBinding({ 'plainshelf.locale': 'zh-Hant' });
    stubHost('DesktopApp', binding);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const { getPreference } = await loadModule();
    binding.GetPreferences.mockRejectedValue(new Error('locked'));

    window.dispatchEvent(new Event('focus'));
    await vi.waitFor(() => expect(binding.GetPreferences.mock.calls.length).toBeGreaterThan(1));
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(getPreference('plainshelf.locale')).toBe('zh-Hant');
  });

  it('is loaded before preference modules read their initial value', async () => {
    runtime.host = 'desktop';
    stubHost('DesktopApp', hostBinding({ 'reader-launch-mode': 'in-window', 'plainshelf.shelf': 'main' }));
    await loadModule();
    const { useReaderLaunchPreference } = await import('@/composables/useReaderLaunchPreference');
    const { getActiveShelfID } = await import('@/api/client');

    expect(useReaderLaunchPreference().mode.value).toBe('in-window');
    expect(getActiveShelfID()).toBe('main');
  });
});
