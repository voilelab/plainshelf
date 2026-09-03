import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { effectScope, nextTick, ref } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  DEFAULT_READER_FONT,
  getReaderFontFamily,
  parseReaderFont,
  READER_FONT_OPTIONS,
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

// The end-to-end case that used to read `font-family` off the rendered reader
// is gone; what it proved beyond the storage round-trip above was that each id
// maps to its own stack and that code text opts out of the reading font. Both
// are pinned here — jsdom resolves no `var()`, so the opt-out is asserted
// against the stylesheet that states it.
describe('reader font reaches the rendered text', () => {
  it('gives each font id its own family stack', () => {
    expect(getReaderFontFamily('system')).toMatch(/^Georgia,/);
    expect(getReaderFontFamily('noto-serif-tc')).toMatch(/^'Noto Serif TC Variable',/);
    expect(getReaderFontFamily('noto-sans-tc')).toMatch(/^'Noto Sans TC Variable',/);
    expect(new Set(READER_FONT_OPTIONS.map((option) => option.cssFamily)).size).toBe(
      READER_FONT_OPTIONS.length
    );
  });

  it('keeps code text on a monospace stack the reading font cannot reach', () => {
    const css = readFileSync(resolve('src/features/reader/styles/reader-content.css'), 'utf8');
    const rules = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)];
    const familyOf = (selector: string) =>
      rules.find((rule) => rule[1].includes(selector))?.[2].match(/font-family:([^;]*)/)?.[1] ?? '';

    // Body text and headings follow the chosen font through the custom property
    // ReaderView sets from getReaderFontFamily.
    expect(familyOf('.reader-text')).toContain('var(--reader-font-family');
    expect(familyOf(':deep(h1)')).toContain('var(--reader-font-family');

    // Code does not: a proportional reading font would mangle indentation.
    expect(familyOf('.reader-md-inline-code')).toContain('monospace');
    expect(familyOf('.reader-md-inline-code')).not.toContain('--reader-font-family');
  });
});
