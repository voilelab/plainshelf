import { describe, expect, it } from 'vitest';
import vectors from './readingPosition.vectors.json';
import {
  measureReadingBlocks,
  offsetAtScroll,
  scrollAtOffset,
  type MeasuredReadingBlocks
} from './readingPosition';

interface ReadingPositionVector {
  kind: 'save' | 'restore';
  chars: number[];
  heights: number[];
  scrollTop: number;
  offset: number;
}

function prefixes(values: number[]): number[] {
  return values.map((_unused, i) => values.slice(0, i).reduce((sum, value) => sum + value, 0));
}

function layout(chars: number[], heights: number[]): MeasuredReadingBlocks {
  const charStarts = prefixes(chars);
  const tops = prefixes(heights);
  return {
    blocks: charStarts.map((charStart, i) => ({ charStart, top: tops[i] })),
    length: chars.reduce((sum, value) => sum + value, 0),
    bottom: heights.reduce((sum, value) => sum + value, 0)
  };
}

const cases = vectors as ReadingPositionVector[];

describe('readingPosition matches the Lean model', () => {
  it('covers every layout the model samples', () => {
    expect(cases.length).toBe(188);
  });

  it.each(cases)('$kind on $heights at scrollTop $scrollTop / offset $offset', (vector) => {
    const measured = layout(vector.chars, vector.heights);
    if (vector.kind === 'save') {
      expect(offsetAtScroll(measured, vector.scrollTop)).toBe(vector.offset);
    } else {
      expect(scrollAtOffset(measured, vector.offset)).toBe(vector.scrollTop);
    }
  });
});

describe('measureReadingBlocks character spans', () => {
  function charStarts(weights: number[], length: number): number[] | undefined {
    return measureReadingBlocks(weights, weights.map((_unused, i) => i * 10), weights.length * 10, length)
      ?.blocks.map((block) => block.charStart);
  }

  it('splits the section in proportion to block weights', () => {
    expect(charStarts([1, 1, 2], 400)).toEqual([0, 100, 200]);
  });

  it('keeps every block at least one character wide', () => {
    expect(charStarts([1000, 1, 1, 1], 4)).toEqual([0, 1, 2, 3]);
    expect(charStarts([0, Number.NaN, 5], 10)).toEqual([0, 1, 2]);
  });

  it('gives up when the section is shorter than its block count', () => {
    expect(charStarts([1, 1, 1], 2)).toBeUndefined();
    expect(charStarts([], 10)).toBeUndefined();
  });
});

describe('measureReadingBlocks', () => {
  it('counts the space above the first block as part of it', () => {
    expect(measureReadingBlocks([1, 1], [24, 124], 224, 10)?.blocks).toEqual([
      { charStart: 0, top: 0 },
      { charStart: 5, top: 124 }
    ]);
  });

  it('rejects a layout with an empty or reversed block', () => {
    expect(measureReadingBlocks([1, 1], [0, 0], 100, 10)).toBeNull();
    expect(measureReadingBlocks([1, 1], [0, 50], 50, 10)).toBeNull();
  });

  it('clamps positions outside the section', () => {
    const measured = layout([10, 10], [100, 100]);
    expect(offsetAtScroll(measured, 5000)).toBe(20);
    expect(scrollAtOffset(measured, -3)).toBe(0);
    expect(scrollAtOffset(measured, 99)).toBe(200);
  });
});
