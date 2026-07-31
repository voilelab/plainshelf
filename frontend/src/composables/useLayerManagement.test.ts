import { describe, expect, it } from 'vitest';
import { movedLayerDestination, renamedLayerDestination } from './useLayerManagement';

describe('renamedLayerDestination', () => {
  it('follows the renamed layer itself', () => {
    expect(renamedLayerDestination('fiction', 'fiction', 'stories')).toBe('stories');
  });

  it('follows a nested layer, keeping the part below the rename', () => {
    expect(renamedLayerDestination('fiction/quiet/river', 'fiction', 'stories')).toBe('stories/quiet/river');
  });

  it('keeps the parent path when renaming a nested layer', () => {
    expect(renamedLayerDestination('a/b/c', 'a/b', 'x')).toBe('a/x/c');
  });

  it('returns undefined when the current layer is unaffected', () => {
    expect(renamedLayerDestination('history', 'fiction', 'stories')).toBeUndefined();
    expect(renamedLayerDestination(undefined, 'fiction', 'stories')).toBeUndefined();
  });

  it('does not treat a name-prefix match as a descendant', () => {
    expect(renamedLayerDestination('fictional', 'fiction', 'stories')).toBeUndefined();
  });
});

describe('movedLayerDestination', () => {
  it('follows the moved layer under its new parent', () => {
    expect(movedLayerDestination('quiet', 'quiet', 'fiction')).toBe('fiction/quiet');
  });

  it('follows a nested layer below the moved one', () => {
    expect(movedLayerDestination('quiet/river', 'quiet', 'fiction')).toBe('fiction/quiet/river');
  });

  it('moves to the root when the target is /', () => {
    expect(movedLayerDestination('fiction/quiet', 'fiction/quiet', '/')).toBe('quiet');
  });

  it('returns undefined when the current layer is unaffected', () => {
    expect(movedLayerDestination('history', 'quiet', 'fiction')).toBeUndefined();
    expect(movedLayerDestination(undefined, 'quiet', 'fiction')).toBeUndefined();
  });

  it('returns undefined for a path with no name segment to move', () => {
    expect(movedLayerDestination('/', '/', 'fiction')).toBeUndefined();
  });

  it('does not treat a name-prefix match as a descendant', () => {
    expect(movedLayerDestination('quietly', 'quiet', 'fiction')).toBeUndefined();
  });
});
