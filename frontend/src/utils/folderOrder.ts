import type { FolderMetadata } from './folders';

export type FolderOrderPlacement = 'before' | 'after';

export function planFolderOrder(
  siblings: string[],
  source: string,
  target: string,
  metadata: FolderMetadata,
  placement: FolderOrderPlacement = 'before'
): { path: string; key: number; version: string }[] {
  if (
    source === target ||
    !siblings.includes(source) ||
    !siblings.includes(target)
  )
    return [];
  const parent = (path: string) => path.split('/').slice(0, -1).join('/');
  if (siblings.some((path) => path === '/' || parent(path) !== parent(source)))
    return [];
  const next = siblings.filter((path) => path !== source);
  next.splice(next.indexOf(target) + (placement === 'after' ? 1 : 0), 0, source);
  if (next.every((path, i) => path === siblings[i])) return [];
  const keys = siblings.map((path) => metadata[path]?.sort_key);
  const strict = keys.every(
    (key, i) =>
      typeof key === 'number' &&
      Number.isFinite(key) &&
      (i === 0 || key > keys[i - 1]!)
  );
  const index = next.indexOf(source);
  const left = metadata[next[index - 1] ?? '']?.sort_key;
  const right = metadata[next[index + 1] ?? '']?.sort_key;
  const key =
    left === undefined
      ? (right ?? 0) - 1024
      : right === undefined
        ? left + 1024
        : left / 2 + right / 2;
  if (
    strict &&
    Number.isFinite(key) &&
    (left === undefined || key > left) &&
    (right === undefined || key < right)
  ) {
    return [{ path: source, key, version: metadata[source]?.version ?? '' }];
  }
  return next.map((path, i) => ({
    path,
    key: (i + 1) * 1024,
    version: metadata[path]?.version ?? ''
  }));
}
