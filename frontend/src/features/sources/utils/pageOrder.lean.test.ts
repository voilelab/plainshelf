import { describe, expect, it } from 'vitest';
import vectors from './pageOrder.vectors.json';
import { dropDestination, movePage, type PageDropSide } from './pageOrder';

interface PageOrderVector {
  length: number;
  from: number;
  target: number;
  side: PageDropSide;
  destination: number;
  result: number[];
}

const cases = vectors as PageOrderVector[];

describe('pageOrder matches the Lean model', () => {
  it('covers every drop on lists up to five pages', () => {
    expect(cases.length).toBe(110);
  });

  it.each(cases)('$length pages, $from onto $target $side', (vector) => {
    const names = Array.from({ length: vector.length }, (_unused, index) => index);
    const destination = dropDestination(vector.from, vector.target, vector.side);
    expect(destination).toBe(vector.destination);
    expect(movePage(names.map(String), vector.from, destination)).toEqual(vector.result.map(String));
  });
});
