import type { Book } from '@/types/book';

export const ROOT_LAYER_PATH = '';

export type LayerTreeNode = {
  name: string;
  path: string;
  children: LayerTreeNode[];
};

export type LayerPathOption = {
  path: string;
  depth: number;
};

export type BooksLayerRoute = {
  path: string;
  query: Record<string, string>;
};

export function normalizeLayerInput(layers?: string | string[] | null): string[] {
  if (!layers) {
    return [];
  }

  const rawLayers = Array.isArray(layers) ? layers : layers.split('/');
  return rawLayers.map((layer) => layer.trim()).filter((layer) => layer.length > 0);
}

export function normalizeLayers(layers?: string[]): string[] {
  return normalizeLayerInput(layers);
}

export function layersToPath(layers?: string[]): string {
  const normalized = normalizeLayers(layers);
  if (normalized.length === 0) {
    return ROOT_LAYER_PATH;
  }
  return normalized.join('/');
}

export function getLayerPath(book: Pick<Book, 'layers'>): string {
  return layersToPath(book.layers);
}

export function layerPathLabel(path: string): string {
  return path || 'No layer';
}

export function normalizeLayerPath(path: string): string {
  const trimmed = path.trim();
  if (trimmed === '' || trimmed === '/') {
    return ROOT_LAYER_PATH;
  }

  const segments = trimmed.split('/').map((segment) => segment.trim()).filter((segment) => segment.length > 0);
  return segments.length === 0 ? ROOT_LAYER_PATH : segments.join('/');
}

export function toComparableLayerPath(path: string): string {
  return normalizeLayerPath(path);
}

export function layerPathEquals(left: string, right: string): boolean {
  return toComparableLayerPath(left) === toComparableLayerPath(right);
}

/**
 * The library route that lists `layerPath` from its first page. Shared by every
 * caller that navigates into a layer (sidebar tree, breadcrumb, post-delete
 * redirect) so they all emit the same canonical `layers` query. The root layer
 * omits the key entirely, which is how LibraryPage reads "all books".
 */
export function booksRouteForLayerPath(layerPath: string): BooksLayerRoute {
  const normalized = normalizeLayerPath(layerPath);
  return normalized === ROOT_LAYER_PATH
    ? { path: '/books', query: { page: '1' } }
    : { path: '/books', query: { layers: normalized, page: '1' } };
}

export function buildLayerTreeNodes(layerPaths: string[]): LayerTreeNode[] {
  const roots: LayerTreeNode[] = [];
  const nodeByPath = new Map<string, LayerTreeNode>();

  for (const inputPath of [...layerPaths].sort((a, b) => a.localeCompare(b))) {
    const normalizedPath = normalizeLayerPath(inputPath);
    if (normalizedPath === ROOT_LAYER_PATH) {
      if (!nodeByPath.has('/')) {
        const rootNode: LayerTreeNode = { name: '/', path: '/', children: [] };
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

  const sortNodes = (nodes: LayerTreeNode[]): void => {
    nodes.sort((a, b) => a.name.localeCompare(b.name));
    for (const node of nodes) {
      sortNodes(node.children);
    }
  };

  sortNodes(roots);
  return roots;
}

/**
 * Flattens a layer tree into a depth-first list of selectable paths. Reads the
 * tree rather than the raw layer list because the tree synthesizes intermediate
 * ancestors that the flat list may omit. The synthetic '/' root node is skipped;
 * callers that need a root entry prepend their own labelled option.
 */
export function flattenLayerTreePaths(nodes: LayerTreeNode[], depth = 0): LayerPathOption[] {
  const options: LayerPathOption[] = [];

  for (const node of nodes) {
    if (node.path === '/') {
      continue;
    }

    options.push({ path: node.path, depth });
    options.push(...flattenLayerTreePaths(node.children, depth + 1));
  }

  return options;
}
