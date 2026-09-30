import { describe, expect, it } from 'vitest';
import { createdFolderDestination, movedFolderDestination, renamedFolderDestination } from './useFolderManagement';

describe('createdFolderDestination', () => {
  it('creates a child beneath the folder whose context menu was used', () => {
    expect(createdFolderDestination('fiction/classics', 'Novellas')).toBe('fiction/classics/Novellas');
  });

  it('creates a top-level folder from the All books context menu', () => {
    expect(createdFolderDestination('/', 'Fiction')).toBe('Fiction');
  });
});

describe('renamedFolderDestination', () => {
  it('follows the renamed folder itself', () => {
    expect(renamedFolderDestination('fiction', 'fiction', 'stories')).toBe('stories');
  });

  it('follows a nested folder, keeping the part below the rename', () => {
    expect(renamedFolderDestination('fiction/quiet/river', 'fiction', 'stories')).toBe('stories/quiet/river');
  });

  it('keeps the parent path when renaming a nested folder', () => {
    expect(renamedFolderDestination('a/b/c', 'a/b', 'x')).toBe('a/x/c');
  });

  it('returns undefined when the current folder is unaffected', () => {
    expect(renamedFolderDestination('history', 'fiction', 'stories')).toBeUndefined();
    expect(renamedFolderDestination(undefined, 'fiction', 'stories')).toBeUndefined();
  });

  it('does not treat a name-prefix match as a descendant', () => {
    expect(renamedFolderDestination('fictional', 'fiction', 'stories')).toBeUndefined();
  });
});

describe('movedFolderDestination', () => {
  it('follows the moved folder under its new parent', () => {
    expect(movedFolderDestination('quiet', 'quiet', 'fiction')).toBe('fiction/quiet');
  });

  it('follows a nested folder below the moved one', () => {
    expect(movedFolderDestination('quiet/river', 'quiet', 'fiction')).toBe('fiction/quiet/river');
  });

  it('moves to the root when the target is /', () => {
    expect(movedFolderDestination('fiction/quiet', 'fiction/quiet', '/')).toBe('quiet');
  });

  it('returns undefined when the current folder is unaffected', () => {
    expect(movedFolderDestination('history', 'quiet', 'fiction')).toBeUndefined();
    expect(movedFolderDestination(undefined, 'quiet', 'fiction')).toBeUndefined();
  });

  it('returns undefined for a path with no name segment to move', () => {
    expect(movedFolderDestination('/', '/', 'fiction')).toBeUndefined();
  });

  it('does not treat a name-prefix match as a descendant', () => {
    expect(movedFolderDestination('quietly', 'quiet', 'fiction')).toBeUndefined();
  });
});
