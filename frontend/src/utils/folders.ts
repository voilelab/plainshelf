import { t } from '@/i18n';
import type { Book } from '@/types/book';

const ROOT_FOLDER_PATH = '';

/**
 * The `folders` query value that filters to books sitting directly at the shelf
 * root. Distinct from ROOT_FOLDER_PATH / a missing query, which mean "all books
 * in every folder".
 */
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

/**
 * Shared by every caller that navigates into a folder (sidebar tree, breadcrumb,
 * post-delete redirect) so they all emit the same canonical `folders` query.
 *
 * Three destinations, not two: an empty path is the unfiltered "All books" view
 * (no `folders` key), `ROOT_FOLDER_FILTER` is the narrower "books sitting
 * directly at the shelf root" filter, and anything else is a normalized folder
 * path. `ROOT_FOLDER_FILTER` must survive verbatim — LibraryPage's
 * `matchesFolder` tells it apart from a missing query.
 */
export function booksRouteForFolderPath(folderPath: string): BooksFolderRoute {
  const trimmed = folderPath.trim();
  const normalized = trimmed === ROOT_FOLDER_FILTER ? ROOT_FOLDER_FILTER : normalizeFolderPath(trimmed);
  return normalized === ROOT_FOLDER_PATH
    ? { path: '/books', query: { page: '1' } }
    : { path: '/books', query: { folders: normalized, page: '1' } };
}

export function buildFolderTreeNodes(folderPaths: string[]): FolderTreeNode[] {
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
    nodes.sort((a, b) => a.name.localeCompare(b.name));
    for (const node of nodes) {
      sortNodes(node.children);
    }
  };

  sortNodes(roots);
  return roots;
}

/**
 * Flattens a folder tree into a depth-first list of selectable paths. Reads the
 * tree rather than the raw folder list because the tree synthesizes intermediate
 * ancestors that the flat list may omit. The synthetic '/' root node is skipped;
 * callers that need a root entry prepend their own labelled option.
 */
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
