import { describe, expect, it } from 'vitest';
import {
  clampTextOffset,
  findMatchOffset,
  matchOffsets,
  paragraphStartOffset,
  replaceAllText,
  replaceTextRange
} from './textEditing';

describe('source editor text operations', () => {
  it('finds non-overlapping matches', () => {
    expect(matchOffsets('aaaa', 'aa')).toEqual([0, 2]);
    expect(matchOffsets('text', '')).toEqual([]);
  });

  it('finds forward and backward matches with wrapping', () => {
    const text = 'one two one';
    expect(findMatchOffset(text, 'one', 0, 0, false)).toBe(0);
    expect(findMatchOffset(text, 'one', 0, 3, false)).toBe(8);
    expect(findMatchOffset(text, 'one', 8, 11, false)).toBe(0);
    expect(findMatchOffset(text, 'one', 8, 8, true)).toBe(0);
    expect(findMatchOffset(text, 'one', 0, 0, true)).toBe(8);
    expect(findMatchOffset(text, 'missing', 0, 0, false)).toBeNull();
  });

  it('clamps and replaces a range', () => {
    expect(replaceTextRange('abcdef', -4, 99, 'x')).toEqual({
      value: 'x',
      selectionStart: 0,
      selectionEnd: 1
    });
    expect(clampTextOffset('abc', -1)).toBe(0);
    expect(clampTextOffset('abc', 8)).toBe(3);
  });

  it('replaces all matches and reports the occurrence count', () => {
    expect(replaceAllText('repeat repeat', 'repeat', 'done')).toEqual({
      value: 'done done',
      occurrences: 2
    });
    expect(replaceAllText('text', '', 'x')).toEqual({ value: 'text', occurrences: 0 });
  });

  it('finds paragraph starts with LF and CRLF blank lines', () => {
    expect(paragraphStartOffset('first\n\nsecond', 13)).toBe(7);
    expect(paragraphStartOffset('first\r\n\r\nsecond', 15)).toBe(9);
    expect(paragraphStartOffset('first\n \t\nsecond', 15)).toBe(9);
  });
});
