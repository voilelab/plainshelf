export function movePage(names: readonly string[], from: number, to: number): string[] {
  const next = [...names];
  if (from < 0 || from >= next.length || to < 0 || to >= next.length || from === to) return next;
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

export function insertionIndex(selectedIndex: number | null, length: number): number {
  if (selectedIndex === null || selectedIndex < 0 || selectedIndex >= length) return length;
  return selectedIndex + 1;
}
