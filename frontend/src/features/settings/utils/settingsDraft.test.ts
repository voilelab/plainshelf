import { describe, expect, it } from 'vitest';
import { buildDefaultSplitConfig, parseReadHistoryLimit } from './settingsDraft';

describe('parseReadHistoryLimit', () => {
  it('accepts a whole, non-negative count', () => {
    expect(parseReadHistoryLimit('50')).toBe(50);
    expect(parseReadHistoryLimit('0')).toBe(0);
  });

  it('rejects negative, fractional and non-numeric input', () => {
    expect(parseReadHistoryLimit('-1')).toBeNull();
    expect(parseReadHistoryLimit('1.5')).toBeNull();
    expect(parseReadHistoryLimit('abc')).toBeNull();
  });

  // Number('') is 0, so clearing the field saves "no limit" rather than
  // reporting an invalid value. Documented here because the number input makes
  // an empty string easy to produce.
  it('treats an empty field as zero', () => {
    expect(parseReadHistoryLimit('')).toBe(0);
  });
});

describe('buildDefaultSplitConfig', () => {
  const draft = { type: 'none' as const, lineCount: '100', regex: '' };

  it('ignores the other fields when splitting is off', () => {
    expect(buildDefaultSplitConfig({ ...draft, lineCount: 'nonsense', regex: '(' })).toEqual({
      ok: true,
      config: { type: 'none' }
    });
  });

  it('builds a line-count config from a positive integer', () => {
    expect(buildDefaultSplitConfig({ ...draft, type: 'line_count', lineCount: '250' })).toEqual({
      ok: true,
      config: { type: 'line_count', line_count: 250 }
    });
  });

  it('rejects a line count that is not a positive integer', () => {
    for (const lineCount of ['0', '-5', 'abc', '']) {
      expect(buildDefaultSplitConfig({ ...draft, type: 'line_count', lineCount })).toEqual({
        ok: false,
        error: 'invalidLineCount'
      });
    }
  });

  it('builds a regex config from a pattern that compiles', () => {
    expect(buildDefaultSplitConfig({ ...draft, type: 'regex', regex: '^Chapter \\d+' })).toEqual({
      ok: true,
      config: { type: 'regex', regex: '^Chapter \\d+' }
    });
  });

  it('rejects a regex that does not compile', () => {
    expect(buildDefaultSplitConfig({ ...draft, type: 'regex', regex: '([' })).toEqual({
      ok: false,
      error: 'invalidRegex'
    });
  });

  it('accepts an empty regex, matching the save path it feeds', () => {
    expect(buildDefaultSplitConfig({ ...draft, type: 'regex', regex: '' })).toEqual({
      ok: true,
      config: { type: 'regex', regex: '' }
    });
  });
});
