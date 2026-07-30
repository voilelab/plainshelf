import { describe, expect, it } from 'vitest';
import { buildLayerTreeNodes, flattenLayerTreePaths } from './layers';

describe('flattenLayerTreePaths', () => {
  it('returns an empty list for an empty tree', () => {
    expect(flattenLayerTreePaths([])).toEqual([]);
  });

  it('skips the synthetic root node', () => {
    expect(flattenLayerTreePaths(buildLayerTreeNodes(['/']))).toEqual([]);
  });

  it('lists each layer depth-first with its nesting depth', () => {
    const options = flattenLayerTreePaths(buildLayerTreeNodes(['/', 'novels', 'novels/scifi', 'zines']));

    expect(options).toEqual([
      { path: 'novels', depth: 0 },
      { path: 'novels/scifi', depth: 1 },
      { path: 'zines', depth: 0 }
    ]);
  });

  it('includes ancestors that the flat layer list omits', () => {
    const options = flattenLayerTreePaths(buildLayerTreeNodes(['novels/scifi/hard']));

    expect(options).toEqual([
      { path: 'novels', depth: 0 },
      { path: 'novels/scifi', depth: 1 },
      { path: 'novels/scifi/hard', depth: 2 }
    ]);
  });

  it('keeps siblings in the tree order', () => {
    const options = flattenLayerTreePaths(buildLayerTreeNodes(['b', 'a', 'a/z', 'a/m']));

    expect(options.map((option) => option.path)).toEqual(['a', 'a/m', 'a/z', 'b']);
  });
});
