import type { FolderMetadata } from './folders';

/** Move before the target; only siblings are legal. Fully keyed groups can
 * update one file; unkeyed or duplicate groups require full normalization.
 * Missing/duplicate keys or exhausted floating point gaps normalize all siblings.
 * Writes are individually versioned, never a cross-file transaction. */
export function planFolderOrder(
  siblings: string[],
  source: string,
  target: string,
  metadata: FolderMetadata
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
  next.splice(next.indexOf(target), 0, source);
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
