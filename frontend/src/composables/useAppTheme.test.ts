/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it } from 'vitest';
import '@/styles.css';
import { initAppTheme, parseAppTheme, useAppTheme } from './useAppTheme';
import { SHELF_BACKDROP_SWATCHES } from '@/storage/shelfBackdrops';

const root = () => document.documentElement;

describe('parseAppTheme', () => {
  it('follows the system when nothing valid is stored', () => {
    expect(parseAppTheme(null)).toBe('system');
    expect(parseAppTheme('sepia')).toBe('system');
  });

  it('keeps a chosen theme', () => {
    expect(parseAppTheme('light')).toBe('light');
    expect(parseAppTheme('dark')).toBe('dark');
  });
});

describe('app theme', () => {
  beforeEach(() => {
    window.localStorage.clear();
    root().removeAttribute('data-theme');
  });

  it('leaves data-theme unset when no theme was chosen', () => {
    initAppTheme();

    expect(root().hasAttribute('data-theme')).toBe(false);
    expect(useAppTheme().theme.value).toBe('system');
  });

  it('applies the theme read back from localStorage', () => {
    window.localStorage.setItem('app-theme', 'dark');

    initAppTheme();

    expect(root().getAttribute('data-theme')).toBe('dark');
    expect(useAppTheme().theme.value).toBe('dark');
  });

  it('stores a chosen theme and applies it', () => {
    useAppTheme().setTheme('light');

    expect(window.localStorage.getItem('app-theme')).toBe('light');
    expect(root().getAttribute('data-theme')).toBe('light');
  });

  it('clears the stored theme when switching back to system', () => {
    useAppTheme().setTheme('dark');
    useAppTheme().setTheme('system');

    expect(window.localStorage.getItem('app-theme')).toBeNull();
    expect(root().hasAttribute('data-theme')).toBe(false);
  });
});

describe('theme tokens in styles.css', () => {
  const token = (name: string) => getComputedStyle(root()).getPropertyValue(name).trim();

  beforeEach(() => {
    window.localStorage.clear();
    root().removeAttribute('data-theme');
  });

  it('keeps the existing light colors when no theme is set', () => {
    expect(token('--bg')).toBe('#f4f6f8');
    expect(token('--text')).toBe('#142033');
  });

  it('changes --bg and --text when the stored dark theme is read back', () => {
    const light = { bg: token('--bg'), text: token('--text') };
    window.localStorage.setItem('app-theme', 'dark');

    initAppTheme();

    expect(token('--bg')).not.toBe(light.bg);
    expect(token('--text')).not.toBe(light.text);
  });

  it('keeps the reading surface on the light tokens under the dark theme', () => {
    const surface = document.createElement('div');
    surface.className = 'reading-surface';
    document.body.append(surface);

    useAppTheme().setTheme('dark');

    expect(token('--bg')).not.toBe('#f4f6f8');
    expect(getComputedStyle(surface).getPropertyValue('--bg').trim()).toBe('#f4f6f8');
    expect(getComputedStyle(surface).getPropertyValue('--surface').trim()).toBe('#ffffff');
    surface.remove();
  });

  it('defines a light and a different dark value for every shelf backdrop swatch', () => {
    for (const swatch of [...SHELF_BACKDROP_SWATCHES, 'scrim']) {
      useAppTheme().setTheme('light');
      const light = token(`--shelf-backdrop-${swatch}`);
      useAppTheme().setTheme('dark');
      const dark = token(`--shelf-backdrop-${swatch}`);

      expect(light, swatch).not.toBe('');
      expect(dark, swatch).not.toBe('');
      expect(dark, swatch).not.toBe(light);
    }
  });

  it('returns to the light values when light is chosen', () => {
    useAppTheme().setTheme('dark');
    useAppTheme().setTheme('light');

    expect(token('--bg')).toBe('#f4f6f8');
    expect(token('--text')).toBe('#142033');
  });
});
