import { effectScope, nextTick, ref } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  DEFAULT_READER_FONT,
  getReaderFontFamily,
  parseReaderFont,
  READER_FONT_OPTIONS,
  useReaderSettings,
  type ReaderFont,
  type ReaderTheme
} from './useReaderSettings';

function stubWindow(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  const localStorage = {
    getItem: vi.fn((key: string) => values.get(key) ?? null),
    setItem: vi.fn((key: string, value: string) => values.set(key, value))
  };
  vi.stubGlobal('window', { localStorage });
  return { values, localStorage };
}

function setupSettings() {
  const scope = effectScope();
  let settings!: ReturnType<typeof useReaderSettings>;
  scope.run(() => {
    settings = useReaderSettings();
  });
  return { scope, settings };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('reader font settings', () => {
  it.each<ReaderFont>(['system', 'noto-serif-tc', 'noto-sans-tc'])('accepts the %s font id', (font) => {
    expect(parseReaderFont(font)).toBe(font);
  });

  it('falls back to the system font for missing or unknown values', () => {
    expect(parseReaderFont(null)).toBe(DEFAULT_READER_FONT);
    expect(parseReaderFont('')).toBe(DEFAULT_READER_FONT);
    expect(parseReaderFont('Comic Sans MS')).toBe(DEFAULT_READER_FONT);
  });

  it('restores both font family and font size without changing existing size behavior', () => {
    stubWindow({
      'reader-font-family': 'noto-sans-tc',
      'reader-font-size': '28'
    });
    const { scope, settings } = setupSettings();

    expect(settings.fontFamily.value).toBe('noto-sans-tc');
    expect(settings.fontSize.value).toBe(28);
    expect(getReaderFontFamily(settings.fontFamily.value)).toContain('Noto Sans TC Variable');

    scope.stop();
  });

  it('persists a font selection on the current device', async () => {
    const { values, localStorage } = stubWindow();
    const { scope, settings } = setupSettings();

    expect(settings.fontFamily.value).toBe('system');
    settings.setFontFamily('noto-serif-tc');
    await nextTick();

    expect(values.get('reader-font-family')).toBe('noto-serif-tc');
    expect(localStorage.setItem).toHaveBeenCalledWith('reader-font-family', 'noto-serif-tc');

    scope.stop();
  });

  it('uses a reactive presentation default until the user chooses a size', async () => {
    const { values } = stubWindow();
    const defaultSize = ref(20);
    const scope = effectScope();
    let settings!: ReturnType<typeof useReaderSettings>;
    scope.run(() => {
      settings = useReaderSettings(defaultSize);
    });

    expect(settings.fontSize.value).toBe(20);
    defaultSize.value = 22;
    await nextTick();
    expect(settings.fontSize.value).toBe(22);
    expect(values.has('reader-font-size')).toBe(false);

    settings.increaseFontSize();
    expect(settings.fontSize.value).toBe(24);
    expect(values.get('reader-font-size')).toBe('24');

    defaultSize.value = 20;
    await nextTick();
    expect(settings.fontSize.value).toBe(24);

    scope.stop();
  });

  it('keeps a stored size over desktop and mobile defaults', async () => {
    stubWindow({ 'reader-font-size': '28' });
    const defaultSize = ref(20);
    const scope = effectScope();
    let settings!: ReturnType<typeof useReaderSettings>;
    scope.run(() => {
      settings = useReaderSettings(defaultSize);
    });

    expect(settings.fontSize.value).toBe(28);
    defaultSize.value = 22;
    await nextTick();
    expect(settings.fontSize.value).toBe(28);

    scope.stop();
  });

  it('syncs font changes to other readers in the same app', async () => {
    stubWindow();
    const first = setupSettings();
    const second = setupSettings();

    first.settings.increaseFontSize();
    first.settings.setFontFamily('noto-serif-tc');
    await nextTick();

    expect(second.settings.fontSize.value).toBe(first.settings.fontSize.value);
    expect(second.settings.fontFamily.value).toBe('noto-serif-tc');

    second.scope.stop();
    first.settings.decreaseFontSize();
    expect(second.settings.fontSize.value).not.toBe(first.settings.fontSize.value);
    first.scope.stop();
  });
});

describe('reader font reaches the rendered text', () => {
  it('gives each font id its own family stack', () => {
    expect(getReaderFontFamily('system')).toMatch(/^Georgia,/);
    expect(getReaderFontFamily('noto-serif-tc')).toMatch(/^'Noto Serif TC Variable',/);
    expect(getReaderFontFamily('noto-sans-tc')).toMatch(/^'Noto Sans TC Variable',/);
    expect(new Set(READER_FONT_OPTIONS.map((option) => option.cssFamily)).size).toBe(
      READER_FONT_OPTIONS.length
    );
  });
});

describe('reader theme settings', () => {
  it('defaults to light when nothing is stored', () => {
    stubWindow();
    const { scope, settings } = setupSettings();

    expect(settings.theme.value).toBe('light');

    scope.stop();
  });

  it.each<ReaderTheme>(['light', 'sepia', 'dark'])('reads back a stored %s theme', async (theme) => {
    const { values } = stubWindow();
    const first = setupSettings();

    first.settings.setTheme(theme);
    await nextTick();
    expect(values.get('reader-theme')).toBe(theme);
    first.scope.stop();

    const reopened = setupSettings();
    expect(reopened.settings.theme.value).toBe(theme);
    reopened.scope.stop();
  });

  it.each(['', 'Dark', 'night', '1'])('falls back to light for the stored value %j', (raw) => {
    stubWindow({ 'reader-theme': raw });
    const { scope, settings } = setupSettings();

    expect(settings.theme.value).toBe('light');

    scope.stop();
  });

  it('leaves stored font settings untouched when the theme changes', async () => {
    const { values } = stubWindow({ 'reader-font-family': 'noto-serif-tc', 'reader-font-size': '26' });
    const { scope, settings } = setupSettings();

    settings.setTheme('dark');
    await nextTick();

    expect(settings.fontFamily.value).toBe('noto-serif-tc');
    expect(settings.fontSize.value).toBe(26);
    expect(values.get('reader-font-family')).toBe('noto-serif-tc');
    expect(values.get('reader-font-size')).toBe('26');

    scope.stop();
  });

  it('syncs the theme to other readers and ignores the app theme key', async () => {
    const { values } = stubWindow({ 'app-theme': 'dark' });
    const first = setupSettings();
    const second = setupSettings();

    expect(first.settings.theme.value).toBe('light');
    first.settings.setTheme('sepia');
    await nextTick();

    expect(second.settings.theme.value).toBe('sepia');
    expect(values.get('app-theme')).toBe('dark');

    first.scope.stop();
    second.scope.stop();
  });
});
