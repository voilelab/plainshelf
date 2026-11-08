/**
 * @vitest-environment jsdom
 */
import { afterEach, describe, expect, it } from 'vitest';
import '@/styles.css';

const root = () => document.documentElement;

function surface(readerTheme?: string): HTMLElement {
  const element = document.createElement('section');
  element.className = 'reading-surface';
  if (readerTheme) element.dataset.readerTheme = readerTheme;
  const text = document.createElement('p');
  element.append(text);
  document.body.append(element);
  return text;
}

const token = (element: Element, name: string) => getComputedStyle(element).getPropertyValue(name).trim();

afterEach(() => {
  document.body.innerHTML = '';
  root().removeAttribute('data-theme');
});

describe('reader theme tokens in styles.css', () => {
  it('keeps the light reader colors when the app theme is dark', () => {
    root().setAttribute('data-theme', 'dark');
    const text = surface();

    expect(token(text, '--reader-paper')).toBe('#fffdf7');
    expect(token(text, '--reader-ink')).toBe('#2f2a22');
    expect(token(text, '--text')).toBe('#142033');
  });

  it('turns the reading surface dark without touching the light app theme', () => {
    root().setAttribute('data-theme', 'light');
    const text = surface('dark');

    expect(token(text, '--reader-paper')).toBe('#1e2024');
    expect(token(text, '--text')).toBe('#e4e9f0');
    expect(token(root(), '--text')).toBe('#142033');
    expect(root().getAttribute('data-theme')).toBe('light');
  });

  it('gives sepia its own paper and ink over the light app tokens', () => {
    const text = surface('sepia');

    expect(token(text, '--reader-paper')).toBe('#f6ecd5');
    expect(token(text, '--reader-ink')).toBe('#3b2e1e');
    expect(token(text, '--text')).toBe('#142033');
  });
});
