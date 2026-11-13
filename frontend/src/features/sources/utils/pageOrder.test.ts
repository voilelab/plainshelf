import { describe, expect, it } from 'vitest';
import { dropDestination, insertionIndex, movePage } from './pageOrder';

describe('movePage', () => {
  it('moves a page forward and backward', () => {
    expect(movePage(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd']);
    expect(movePage(['a', 'b', 'c', 'd'], 3, 1)).toEqual(['a', 'd', 'b', 'c']);
  });

  it('keeps duplicate names apart by index', () => {
    expect(movePage(['a', 'b', 'a'], 2, 0)).toEqual(['a', 'a', 'b']);
  });

  it('returns an unchanged copy for out-of-range or same-slot moves', () => {
    const names = ['a', 'b'];
    expect(movePage(names, 0, 5)).toEqual(names);
    expect(movePage(names, 1, 1)).toEqual(names);
    expect(movePage(names, 0, 1)).not.toBe(names);
  });
});

describe('insertionIndex', () => {
  it('inserts after the selected page or at the end', () => {
    expect(insertionIndex(0, 3)).toBe(1);
    expect(insertionIndex(2, 3)).toBe(3);
    expect(insertionIndex(null, 3)).toBe(3);
    expect(insertionIndex(7, 3)).toBe(3);
  });
});

describe('dropDestination', () => {
  it('lands before or after the target whichever way the page travels', () => {
    expect(dropDestination(8, 2, 'before')).toBe(2);
    expect(dropDestination(8, 2, 'after')).toBe(3);
    expect(dropDestination(2, 8, 'before')).toBe(7);
    expect(dropDestination(2, 8, 'after')).toBe(8);
  });

  it('stays in place when dropped beside itself', () => {
    expect(dropDestination(3, 3, 'before')).toBe(3);
    expect(dropDestination(3, 3, 'after')).toBe(3);
    expect(dropDestination(3, 4, 'before')).toBe(3);
    expect(dropDestination(3, 2, 'after')).toBe(3);
  });
});
