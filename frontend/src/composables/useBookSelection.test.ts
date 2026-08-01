import { describe, expect, it } from 'vitest';
import { effectScope } from 'vue';
import { useBookSelection } from './useBookSelection';

function createSelection() {
  const scope = effectScope();
  const selection = scope.run(() => useBookSelection())!;
  return { selection, stop: () => scope.stop() };
}

describe('useBookSelection', () => {
  it('toggles ids and clears the anchor', () => {
    const { selection, stop } = createSelection();
    selection.toggle('a');
    selection.toggle('b');
    expect([...selection.selectedIds.value]).toEqual(['a', 'b']);
    expect(selection.anchorId.value).toBe('b');
    selection.toggle('a');
    expect([...selection.selectedIds.value]).toEqual(['b']);
    selection.clear();
    expect(selection.selectedIds.value.size).toBe(0);
    expect(selection.anchorId.value).toBeNull();
    stop();
  });

  it('adds a range using the visible order', () => {
    const { selection, stop } = createSelection();
    selection.toggle('b');
    selection.selectRange(['a', 'b', 'c', 'd'], 'd');
    expect([...selection.selectedIds.value]).toEqual(['b', 'c', 'd']);
    stop();
  });

  it('updates the range anchor on direct toggles while keeping it for shift ranges', () => {
    const { selection, stop } = createSelection();
    selection.toggle('a');
    selection.toggle('c');
    expect(selection.anchorId.value).toBe('c');
    selection.selectRange(['a', 'b', 'c', 'd'], 'a');
    expect(selection.selectedIds.value).toEqual(new Set(['a', 'b', 'c']));
    expect(selection.anchorId.value).toBe('c');
    stop();
  });

  it('falls back to toggling when the anchor is not visible', () => {
    const { selection, stop } = createSelection();
    selection.toggle('elsewhere');
    selection.selectRange(['a', 'b'], 'b');
    expect(selection.selectedIds.value).toEqual(new Set(['elsewhere', 'b']));
    expect(selection.anchorId.value).toBe('b');
    stop();
  });

  it('selects only the supplied page and replaces prior ids', () => {
    const { selection, stop } = createSelection();
    selection.toggle('old');
    selection.selectAll(['page-1', 'page-2']);
    expect(selection.selectedIds.value).toEqual(new Set(['page-1', 'page-2']));
    stop();
  });
});
