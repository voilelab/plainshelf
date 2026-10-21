import { FolderHttpError, FolderMetaConflictError } from '../folderErrors';
import { mockBooks } from './books';
import { normalizeFolderInput, normalizeFolderPath } from '@/utils/folders';


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

const mockFolderIcons = new Map<string, { icon: string; version: string; sort_key?: number }>();
let mockFolderMetaVersion = 0;

export function getMockFolderMeta(): Record<string, { icon: string; version: string; readOnly: boolean; sort_key?: number }> {
  const folders: Record<string, { icon: string; version: string; readOnly: boolean; sort_key?: number }> = {};
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
  const current = mockFolderIcons.get(path);
  if (icon === '' && current?.sort_key === undefined) {
    mockFolderIcons.delete(path);
    return;
  }
  mockFolderMetaVersion += 1;
  mockFolderIcons.set(path, { ...current, icon, version: String(mockFolderMetaVersion) });
}

export function setMockFolderSortKey(path: string, sort_key: number, version: string): void {
  if (!mockFolders.has(path)) throw new FolderHttpError('folder not found');
  const current = mockFolderIcons.get(path);
  if ((current?.version ?? '') !== version) {
    throw new FolderMetaConflictError('folder.json changed since it was read; read it again and retry');
  }
  mockFolderMetaVersion += 1;
  mockFolderIcons.set(path, {icon: current?.icon ?? '', sort_key, version: String(mockFolderMetaVersion)});
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

function moveMockSubtree(from: string, to: string): void {
  for (const folder of getMockFolders()) {
    if (folder === from || folder.startsWith(`${from}/`)) {
      mockFolders.delete(folder);
      mockFolders.add(replaceFolderPrefix(folder, from, to));
    }
  }

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
