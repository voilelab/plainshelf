import { t } from '@/i18n';
import type { Book } from '@/types/book';

const ROOT_FOLDER_PATH = '';

export const ROOT_FOLDER_FILTER = '/';

type FolderTreeNode = {
  name: string;
  path: string;
  children: FolderTreeNode[];
};

type FolderPathOption = {
  path: string;
  depth: number;
};

type BooksFolderRoute = {
  path: string;
  query: Record<string, string>;
};

export function normalizeFolderInput(folders?: string | string[] | null): string[] {
  if (!folders) {
    return [];
  }

  const rawFolders = Array.isArray(folders) ? folders : folders.split('/');
  return rawFolders.map((folder) => folder.trim()).filter((folder) => folder.length > 0);
}

function normalizeFolders(folders?: string[]): string[] {
  return normalizeFolderInput(folders);
}

function foldersToPath(folders?: string[]): string {
  const normalized = normalizeFolders(folders);
  if (normalized.length === 0) {
    return ROOT_FOLDER_PATH;
  }
  return normalized.join('/');
}

export function getFolderPath(book: Pick<Book, 'folders'>): string {
  return foldersToPath(book.folders);
}

export function folderPathLabel(path: string): string {
  return path || t('bookCollection.noFolder');
}

export function normalizeFolderPath(path: string): string {
  const trimmed = path.trim();
  if (trimmed === '' || trimmed === '/') {
    return ROOT_FOLDER_PATH;
  }

  const segments = trimmed.split('/').map((segment) => segment.trim()).filter((segment) => segment.length > 0);
  return segments.length === 0 ? ROOT_FOLDER_PATH : segments.join('/');
}

function toComparableFolderPath(path: string): string {
  return normalizeFolderPath(path);
}

export function folderPathEquals(left: string, right: string): boolean {
  return toComparableFolderPath(left) === toComparableFolderPath(right);
}

export function booksRouteForFolderPath(folderPath: string): BooksFolderRoute {
  const trimmed = folderPath.trim();
  const normalized = trimmed === ROOT_FOLDER_FILTER ? ROOT_FOLDER_FILTER : normalizeFolderPath(trimmed);
  return normalized === ROOT_FOLDER_PATH
    ? { path: '/books', query: { page: '1' } }
    : { path: '/books', query: { folders: normalized, page: '1' } };
}

export type FolderMetadata = Record<
  string,
  { sort_key?: number; version: string; read_only?: boolean }
>;

function compareFolderKeys(
  a: string,
  b: string,
  metadata: FolderMetadata
): number {
  const x = metadata[a]?.sort_key,
    y = metadata[b]?.sort_key;
  const hasX = typeof x === 'number' && Number.isFinite(x);
  const hasY = typeof y === 'number' && Number.isFinite(y);
  if (hasX !== hasY) return hasX ? -1 : 1;
  return hasX && hasY ? x! - y! : 0;
}

export function sortFolderPaths(paths: string[], metadata: FolderMetadata): string[] {
  if (!paths.some(path => Number.isFinite(metadata[path]?.sort_key))) return [...paths];
  const present = new Set(paths);
  const children = new Map<string, string[]>();
  const seen = new Set<string>();
  function add(path: string): void {
    if (!path || path === '/' || seen.has(path)) return;
    const parent = path.split('/').slice(0, -1).join('/');
    add(parent);
    seen.add(path);
    const siblings = children.get(parent) ?? [];
    siblings.push(path);
    children.set(parent, siblings);
  }
  paths.forEach(add);
  const result: string[] = paths.filter(path => path === '/' || path === '');
  function visit(parent: string): void {
    const siblings = children.get(parent) ?? [];
    siblings.sort((a, b) => compareFolderKeys(a, b, metadata));
    for (const path of siblings) {
      if (present.has(path)) result.push(path);
      visit(path);
    }
  }
  visit('');
  return result;
}

export function buildFolderTreeNodes(
  folderPaths: string[],
  metadata: FolderMetadata = {}
): FolderTreeNode[] {
  const roots: FolderTreeNode[] = [];
  const nodeByPath = new Map<string, FolderTreeNode>();

  for (const inputPath of [...folderPaths].sort((a, b) => a.localeCompare(b))) {
    const normalizedPath = normalizeFolderPath(inputPath);
    if (normalizedPath === ROOT_FOLDER_PATH) {
      if (!nodeByPath.has('/')) {
        const rootNode: FolderTreeNode = { name: '/', path: '/', children: [] };
        nodeByPath.set('/', rootNode);
        roots.push(rootNode);
      }
      continue;
    }

    const segments = normalizedPath.split('/').filter((segment) => segment.length > 0);
    let parentPath = '';
    let siblings = roots;

    for (const segment of segments) {
      const segmentPath = parentPath.length > 0 ? `${parentPath}/${segment}` : segment;
      let node = nodeByPath.get(segmentPath);

      if (!node) {
        node = {
          name: segment,
          path: segmentPath,
          children: []
        };
        nodeByPath.set(segmentPath, node);
        siblings.push(node);
      }

      parentPath = segmentPath;
      siblings = node.children;
    }
  }

  const sortNodes = (nodes: FolderTreeNode[]): void => {
    nodes.sort((a, b) =>
      compareFolderKeys(a.path, b.path, metadata) || a.name.localeCompare(b.name)
    );
    for (const node of nodes) {
      sortNodes(node.children);
    }
  };

  sortNodes(roots);
  return roots;
}

export function flattenFolderTreePaths(nodes: FolderTreeNode[], depth = 0): FolderPathOption[] {
  const options: FolderPathOption[] = [];

  for (const node of nodes) {
    if (node.path === '/') {
      continue;
    }

    options.push({ path: node.path, depth });
    options.push(...flattenFolderTreePaths(node.children, depth + 1));
  }

  return options;
}
