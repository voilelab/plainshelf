import { LayerHttpError } from '../layerErrors';
import { mockBooks } from './books';
import { normalizeLayerInput, normalizeLayerPath } from '@/utils/layers';

// In-memory layer tree for VITE_USE_MOCK_API, seeded from the mock books so the
// two agree on startup. The gate that reaches for it lives in api/client.ts
// (isMockApiMode); nothing here is reachable in a real build.

function pathFromLayers(layers: string[] = []): string {
  const segments = layers.map((s) => s.trim()).filter((s) => s.length > 0);
  return segments.length === 0 ? '/' : segments.join('/');
}

function replaceLayerPrefix(path: string, oldPrefix: string, newPrefix: string): string {
  if (path === oldPrefix) {
    return newPrefix;
  }
  if (path.startsWith(`${oldPrefix}/`)) {
    return `${newPrefix}${path.slice(oldPrefix.length)}`;
  }
  return path;
}

function deriveMockLayersFromBooks(): string[] {
  const set = new Set<string>();
  set.add('/');

  for (const book of mockBooks) {
    const path = pathFromLayers(book.layers);
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

const mockLayers = new Set<string>(deriveMockLayersFromBooks());

export function getMockLayers(): string[] {
  return Array.from(mockLayers).sort((a, b) => a.localeCompare(b));
}

export function addMockLayer(path: string): void {
  mockLayers.add('/');

  const normalized = normalizeLayerPath(path);
  if (!normalized) {
    return;
  }

  const segments = normalized.split('/').filter((segment) => segment.length > 0);
  for (let i = 1; i <= segments.length; i += 1) {
    mockLayers.add(segments.slice(0, i).join('/'));
  }
}

/** Moves a subtree and drags the mock books' layer paths along with it. */
function moveMockSubtree(from: string, to: string): void {
  for (const layer of getMockLayers()) {
    if (layer === from || layer.startsWith(`${from}/`)) {
      mockLayers.delete(layer);
      mockLayers.add(replaceLayerPrefix(layer, from, to));
    }
  }

  for (const book of mockBooks) {
    const currentPath = pathFromLayers(book.layers);
    const nextPath = replaceLayerPrefix(currentPath, from, to);
    if (nextPath !== currentPath) {
      book.layers = normalizeLayerInput(nextPath);
    }
  }
}

export function renameMockLayer(normalized: string, nextPath: string): void {
  if (mockLayers.has(nextPath)) {
    throw new LayerHttpError('Layer already exists');
  }

  moveMockSubtree(normalized, nextPath);
}

export function moveMockLayer(normalized: string, target: string, destination: string): void {
  if (!mockLayers.has(target || '/')) {
    throw new LayerHttpError('Target layer does not exist');
  }
  if (mockLayers.has(destination)) {
    throw new LayerHttpError('Target layer already contains a layer with this name');
  }

  moveMockSubtree(normalized, destination);
}

export function deleteMockLayer(normalized: string): void {
  const hasBooks = mockBooks.some((book) => pathFromLayers(book.layers) === normalized);
  const hasChildren = getMockLayers().some(
    (path) => path !== normalized && path.startsWith(`${normalized}/`)
  );

  if (hasBooks || hasChildren) {
    throw new LayerHttpError('Cannot delete this layer because it is not empty.');
  }

  mockLayers.delete(normalized);
}
