import { describe, expect, it } from 'vitest';

import { clampTextOffset, paragraphStartOffset } from './textEditing';

describe('clampTextOffset', () => {
  it('keeps offsets inside the text', () => {
    expect(clampTextOffset('abcd', -3)).toBe(0);
    expect(clampTextOffset('abcd', 2)).toBe(2);
    expect(clampTextOffset('abcd', 9)).toBe(4);
  });
});

describe('paragraphStartOffset', () => {
  it('returns zero inside the first paragraph', () => {
    expect(paragraphStartOffset('first line\nstill first\n\nsecond', 5)).toBe(0);
  });

  it('starts after the blank line that precedes the cursor', () => {
    const text = 'first\n\nsecond paragraph\n\nthird';
    expect(paragraphStartOffset(text, text.indexOf('second') + 3)).toBe(text.indexOf('second'));
    expect(paragraphStartOffset(text, text.indexOf('third') + 1)).toBe(text.indexOf('third'));
  });

  it('treats a whitespace-only line and CRLF as a paragraph break', () => {
    const text = 'first\r\n \r\nsecond';
    expect(paragraphStartOffset(text, text.length)).toBe(text.indexOf('second'));
  });
});
