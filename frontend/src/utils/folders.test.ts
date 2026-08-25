import { describe, expect, it } from 'vitest';
import {
  ROOT_FOLDER_FILTER,
  booksRouteForFolderPath,
  buildFolderTreeNodes,
  flattenFolderTreePaths
} from './folders';

describe('booksRouteForFolderPath', () => {
  it('lists a nested folder from its first page', () => {
    expect(booksRouteForFolderPath('novels/scifi')).toEqual({
      path: '/books',
      query: { folders: 'novels/scifi', page: '1' }
    });
  });

  it('omits the folders key for the unfiltered All books view', () => {
    expect(booksRouteForFolderPath('')).toEqual({ path: '/books', query: { page: '1' } });
    expect(booksRouteForFolderPath('  ')).toEqual({ path: '/books', query: { page: '1' } });
  });

  it('keeps the root-folder filter, which is not the same as All books', () => {
    expect(booksRouteForFolderPath(ROOT_FOLDER_FILTER)).toEqual({
      path: '/books',
      query: { folders: '/', page: '1' }
    });
  });

  it('normalizes stray slashes and whitespace', () => {
    expect(booksRouteForFolderPath('/novels//scifi/ ')).toEqual({
      path: '/books',
      query: { folders: 'novels/scifi', page: '1' }
    });
  });
});

describe('flattenFolderTreePaths', () => {
  it('returns an empty list for an empty tree', () => {
    expect(flattenFolderTreePaths([])).toEqual([]);
  });

  it('skips the synthetic root node', () => {
    expect(flattenFolderTreePaths(buildFolderTreeNodes(['/']))).toEqual([]);
  });

  it('lists each folder depth-first with its nesting depth', () => {
    const options = flattenFolderTreePaths(buildFolderTreeNodes(['/', 'novels', 'novels/scifi', 'zines']));

    expect(options).toEqual([
      { path: 'novels', depth: 0 },
      { path: 'novels/scifi', depth: 1 },
      { path: 'zines', depth: 0 }
    ]);
  });

  it('includes ancestors that the flat folder list omits', () => {
    const options = flattenFolderTreePaths(buildFolderTreeNodes(['novels/scifi/hard']));

    expect(options).toEqual([
      { path: 'novels', depth: 0 },
      { path: 'novels/scifi', depth: 1 },
      { path: 'novels/scifi/hard', depth: 2 }
    ]);
  });

  it('keeps siblings in the tree order', () => {
    const options = flattenFolderTreePaths(buildFolderTreeNodes(['b', 'a', 'a/z', 'a/m']));

    expect(options.map((option) => option.path)).toEqual(['a', 'a/m', 'a/z', 'b']);
  });
});
