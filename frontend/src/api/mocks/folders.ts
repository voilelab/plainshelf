import { FolderHttpError, FolderMetaConflictError } from '../folderErrors';
import { mockBooks } from './books';
import { normalizeFolderInput, normalizeFolderPath } from '@/utils/folders';

// In-memory folder tree for VITE_USE_MOCK_API, seeded from the mock books so the
// two agree on startup. The gate that reaches for it lives in api/client.ts
// (isMockApiMode); nothing here is reachable in a real build.

function pathFromFolders(folders: string[] = []): string {
  const segments = folders.map((s) => s.trim()).filter((s) => s.length > 0);
  return segments.length === 0 ? '/' : segments.join('/');
}

function replaceFolderPrefix(path: string, oldPrefix: string, newPrefix: string): string {
  if (path === oldPrefix) {
    return newPrefix;
  }
  if (path.startsWith(`${oldPrefix}/`)) {
    return `${newPrefix}${path.slice(oldPrefix.length)}`;
  }
  return path;
}

function deriveMockFoldersFromBooks(): string[] {
  const set = new Set<string>();
  set.add('/');

  for (const book of mockBooks) {
    const path = pathFromFolders(book.folders);
    set.add(path);

    if (path !== '/') {
      const segments = path.split('/');
      for (let i = 1; i <= segments.length; i += 1) {
        set.add(segments.slice(0, i).join('/'));
      }
    }
  }

  return Array.from(set).sort((a, b) => a.localeCompare(b));
}

const mockFolders = new Set<string>(deriveMockFoldersFromBooks());

// Icons by folder path, standing in for each folder's folder.json. A version
// counter plays the content hash the server hands out, so a stale write can be
// refused the same way.
const mockFolderIcons = new Map<string, { icon: string; version: string }>();
let mockFolderMetaVersion = 0;

export function getMockFolderMeta(): Record<string, { icon: string; version: string; readOnly: boolean }> {
  const folders: Record<string, { icon: string; version: string; readOnly: boolean }> = {};
  for (const [path, meta] of mockFolderIcons) {
    folders[path] = { ...meta, readOnly: false };
  }
  return folders;
}

export function setMockFolderIcon(path: string, icon: string, version: string): void {
  if (!mockFolders.has(path)) {
    throw new FolderHttpError('folder not found');
  }
  if ((mockFolderIcons.get(path)?.version ?? '') !== version) {
    throw new FolderMetaConflictError('folder.json changed since it was read; read it again and retry');
  }
  if (icon === '') {
    mockFolderIcons.delete(path);
    return;
  }
  mockFolderMetaVersion += 1;
  mockFolderIcons.set(path, { icon, version: String(mockFolderMetaVersion) });
}

export function getMockFolders(): string[] {
  return Array.from(mockFolders).sort((a, b) => a.localeCompare(b));
}

export function addMockFolder(path: string): void {
  mockFolders.add('/');

  const normalized = normalizeFolderPath(path);
  if (!normalized) {
    return;
  }

  const segments = normalized.split('/').filter((segment) => segment.length > 0);
  for (let i = 1; i <= segments.length; i += 1) {
    mockFolders.add(segments.slice(0, i).join('/'));
  }
}

/** Moves a subtree and drags the mock books' folder paths along with it. */
function moveMockSubtree(from: string, to: string): void {
  for (const folder of getMockFolders()) {
    if (folder === from || folder.startsWith(`${from}/`)) {
      mockFolders.delete(folder);
      mockFolders.add(replaceFolderPrefix(folder, from, to));
    }
  }

  // folder.json lives in the directory, so it moves with it.
  for (const [path, meta] of [...mockFolderIcons]) {
    const nextPath = replaceFolderPrefix(path, from, to);
    if (nextPath !== path) {
      mockFolderIcons.delete(path);
      mockFolderIcons.set(nextPath, meta);
    }
  }

  for (const book of mockBooks) {
    const currentPath = pathFromFolders(book.folders);
    const nextPath = replaceFolderPrefix(currentPath, from, to);
    if (nextPath !== currentPath) {
      book.folders = normalizeFolderInput(nextPath);
    }
  }
}

export function renameMockFolder(normalized: string, nextPath: string): void {
  if (mockFolders.has(nextPath)) {
    throw new FolderHttpError('Folder already exists');
  }

  moveMockSubtree(normalized, nextPath);
}

export function moveMockFolder(normalized: string, target: string, destination: string): void {
  if (!mockFolders.has(target || '/')) {
    throw new FolderHttpError('Target folder does not exist');
  }
  if (mockFolders.has(destination)) {
    throw new FolderHttpError('Target folder already contains a folder with this name');
  }

  moveMockSubtree(normalized, destination);
}

export function deleteMockFolder(normalized: string): void {
  const hasBooks = mockBooks.some((book) => pathFromFolders(book.folders) === normalized);
  const hasChildren = getMockFolders().some(
    (path) => path !== normalized && path.startsWith(`${normalized}/`)
  );

  if (hasBooks || hasChildren) {
    throw new FolderHttpError('Cannot delete this folder because it is not empty.');
  }

  mockFolders.delete(normalized);
  mockFolderIcons.delete(normalized);
}
