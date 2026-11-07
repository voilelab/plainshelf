import { describe, expect, it } from 'vitest';
import { clampComicZoom, comicPageSize, parseComicFit } from './comicPageSize';

const page = { width: 800, height: 1200 };
const viewport = { width: 1280, height: 600 };

describe('comicPageSize', () => {
  it.each([
    ['fit-height', 100, { width: 400, height: 600 }],
    ['fit-width', 100, { width: 1280, height: 1920 }],
    ['original', 100, { width: 800, height: 1200 }],
    ['fit-height', 250, { width: 1000, height: 1500 }],
    ['original', 50, { width: 400, height: 600 }]
  ] as const)('sizes a portrait page with %s at %i%%', (fit, zoom, expected) => {
    expect(comicPageSize(page, viewport, fit, zoom)).toEqual(expected);
  });

  it('fits a landscape page by width in fit-height mode so the default never overflows', () => {
    expect(comicPageSize({ width: 2000, height: 1000 }, { width: 390, height: 700 }, 'fit-height', 100)).toEqual({ width: 390, height: 195 });
  });

  it('returns null before the image or viewport has a size', () => {
    expect(comicPageSize({ width: 0, height: 0 }, viewport, 'fit-width', 100)).toBeNull();
    expect(comicPageSize(page, { width: 0, height: 0 }, 'fit-width', 100)).toBeNull();
  });

  it('parses stored preferences with safe defaults', () => {
    expect(parseComicFit('fit-width')).toBe('fit-width');
    expect(parseComicFit('cover')).toBe('fit-height');
    expect(parseComicFit(null)).toBe('fit-height');
    expect(clampComicZoom(1000)).toBe(300);
    expect(clampComicZoom(10)).toBe(50);
    expect(clampComicZoom(130)).toBe(125);
    expect(clampComicZoom(Number.NaN)).toBe(100);
  });
});
