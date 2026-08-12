import { effectScope, nextTick, ref } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  DEFAULT_READER_FONT,
  getReaderFontFamily,
  parseReaderFont,
  useReaderSettings,
  type ReaderFont
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
});
