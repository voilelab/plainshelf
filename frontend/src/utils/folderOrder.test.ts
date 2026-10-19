import { describe, expect, it } from 'vitest';
import { planFolderOrder } from './folderOrder';
import { buildFolderTreeNodes, sortFolderPaths } from './folders';

describe('folder ordering', () => {
  it('keeps legacy locale order without keys and compares only siblings', () => {
    const paths = ['b', 'a', 'a/z', 'a/m'];
    expect(buildFolderTreeNodes(paths).map((n) => n.path)).toEqual(['a', 'b']);
    const meta = {
      b: { sort_key: 0, version: '' },
      'a/z': { sort_key: -1, version: '' }
    };
    const tree = buildFolderTreeNodes(paths, meta);
    expect(tree.map((n) => n.path)).toEqual(['b', 'a']);
    expect(tree[1]!.children.map((n) => n.path)).toEqual(['a/z', 'a/m']);
    expect(sortFolderPaths(['a', 'a/m', 'a/z', 'b'], meta)).toEqual([
      'b',
      'a/z',
      'a/m',
      'a'
    ]);
  });
  it('keeps duplicate keys stable and ignores invalid numbers', () => {
    const metadata = {
      a: { version: '', sort_key: 0 },
      b: { version: '', sort_key: 0 },
      c: { version: '', sort_key: NaN }
    };
    expect(sortFolderPaths(['b', 'c', 'a'], metadata)).toEqual(['b', 'a', 'c']);
  });
  it('uses one versioned midpoint when gaps exist', () => {
    expect(
      planFolderOrder(['a', 'b', 'c'], 'c', 'b', {
        a: { version: 'a', sort_key: -10 },
        b: { version: 'b', sort_key: 0 },
        c: { version: 'c', sort_key: 10 }
      })
    ).toEqual([{ path: 'c', key: -5, version: 'c' }]);
  });
  it('normalizes missing and duplicate keys in the requested order, and refuses cross-parent drops', () => {
    expect(planFolderOrder(['a', 'b'], 'b', 'a', {})).toEqual([
      { path: 'b', key: 1024, version: '' },
      { path: 'a', key: 2048, version: '' }
    ]);
    expect(
      planFolderOrder(['a', 'b'], 'b', 'a', {
        a: { sort_key: 0, version: '' },
        b: { sort_key: 0, version: '' }
      })
    ).toHaveLength(2);
    expect(planFolderOrder(['x/a', 'x/b'], 'a', 'x/b', {})).toEqual([]);
  });
});
