import { describe, expect, it } from 'vitest';
import { normalizeEpubImportPreset, normalizeEpubImportStrategy } from './epubStrategy';

describe('normalizeEpubImportPreset', () => {
  it('keeps the known presets', () => {
    expect(normalizeEpubImportPreset('markdown')).toBe('markdown');
    expect(normalizeEpubImportPreset('plain')).toBe('plain');
  });

  it('falls back to markdown for anything else', () => {
    expect(normalizeEpubImportPreset('custom')).toBe('markdown');
    expect(normalizeEpubImportPreset('')).toBe('markdown');
    expect(normalizeEpubImportPreset(undefined)).toBe('markdown');
    expect(normalizeEpubImportPreset(null)).toBe('markdown');
    expect(normalizeEpubImportPreset(3)).toBe('markdown');
  });
});

describe('normalizeEpubImportStrategy', () => {
  it('passes through a complete strategy', () => {
    expect(normalizeEpubImportStrategy({ preset: 'plain', include_description: false })).toEqual({
      preset: 'plain',
      include_description: false
    });
  });

  it('fills in missing fields from the default', () => {
    expect(normalizeEpubImportStrategy({ preset: 'plain' })).toEqual({
      preset: 'plain',
      include_description: true
    });
    expect(normalizeEpubImportStrategy({ include_description: false })).toEqual({
      preset: 'markdown',
      include_description: false
    });
  });

  it('ignores unknown fields rather than forwarding them to the server', () => {
    expect(
      normalizeEpubImportStrategy({ preset: 'markdown', include_description: true, template: 'x' })
    ).toEqual({ preset: 'markdown', include_description: true });
  });

  it('returns the default for values that are not objects', () => {
    const fallback = { preset: 'markdown', include_description: true };
    expect(normalizeEpubImportStrategy(null)).toEqual(fallback);
    expect(normalizeEpubImportStrategy(undefined)).toEqual(fallback);
    expect(normalizeEpubImportStrategy('markdown')).toEqual(fallback);
    expect(normalizeEpubImportStrategy(42)).toEqual(fallback);
  });
});
