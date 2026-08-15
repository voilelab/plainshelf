import { computed, ref } from 'vue';

export function useBookSelection() {
  const selectedIds = ref<Set<string>>(new Set());
  const anchorId = ref<string | null>(null);
  const active = computed(() => selectedIds.value.size > 0);
  const count = computed(() => selectedIds.value.size);

  function commit(next: Set<string>): void {
    selectedIds.value = next;
  }

  function clear(): void {
    commit(new Set());
    anchorId.value = null;
  }

  function replace(ids: Iterable<string>): void {
    const next = new Set(ids);
    commit(next);
    anchorId.value = next.size > 0 ? [...next][next.size - 1] : null;
  }

  function toggle(id: string): void {
    const next = new Set(selectedIds.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    commit(next);
    anchorId.value = id;
  }

  function selectRange(orderedIds: string[], targetId: string): void {
    const anchorIndex = anchorId.value ? orderedIds.indexOf(anchorId.value) : -1;
    const targetIndex = orderedIds.indexOf(targetId);
    if (anchorIndex < 0 || targetIndex < 0) {
      toggle(targetId);
      return;
    }

    const start = Math.min(anchorIndex, targetIndex);
    const end = Math.max(anchorIndex, targetIndex);
    const next = new Set(selectedIds.value);
    for (const id of orderedIds.slice(start, end + 1)) next.add(id);
    commit(next);
  }

  function selectAll(orderedIds: string[]): void {
    replace(orderedIds);
  }

  function isSelected(id: string): boolean {
    return selectedIds.value.has(id);
  }

  return { selectedIds, anchorId, active, count, clear, replace, toggle, selectRange, selectAll, isSelected };
}
