import { mockBooks } from './books';
import { ApiError, buildShelfApiPath, fetchJson, isMockApiMode } from './client';
import { normalizeLayerPath } from '../utils/layers';

function delay<T>(value: T, ms = 240): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), ms);
  });
}

function normalizeLayerValue(value: unknown): string | null {
  if (typeof value === 'string') {
    const normalized = normalizeLayerPath(value);
    return normalized.length > 0 ? normalized : '/';
  }

  if (Array.isArray(value)) {
    const segments = value
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter((item) => item.length > 0);

    if (segments.length === 0) {
      return '/';
    }
    return normalizeLayerPath(segments.join('/'));
  }

  return null;
}

function layersFromPath(path: string): string[] {
  const normalized = normalizeLayerPath(path);
  if (!normalized || normalized === '/') {
    return [];
  }
  return normalized.split('/').filter((segment) => segment.length > 0);
}

function pathFromLayers(layers: string[] = []): string {
  const segments = layers.map((s) => s.trim()).filter((s) => s.length > 0);
  return segments.length === 0 ? '/' : segments.join('/');
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

function getMockLayers(): string[] {
  return Array.from(mockLayers).sort((a, b) => a.localeCompare(b));
}

function addMockLayer(path: string): void {
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

function deleteMockLayer(path: string): void {
  mockLayers.delete(path);
}

function encodeLayerPath(path: string): string {
  return path
    .split('/')
    .filter((segment) => segment.length > 0)
    .map((segment) => encodeURIComponent(segment))
    .join('/');
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

function syncMockBooksLayerPrefix(oldPrefix: string, newPrefix: string): void {
  for (const book of mockBooks) {
    const currentPath = pathFromLayers(book.layers);
    const nextPath = replaceLayerPrefix(currentPath, oldPrefix, newPrefix);
    if (nextPath !== currentPath) {
      book.layers = layersFromPath(nextPath);
    }
  }
}

class LayerHttpError extends Error {}

export async function getLayers(): Promise<string[]> {
  if (isMockApiMode()) {
    return delay(getMockLayers());
  }

  const data: unknown = await fetchJson<unknown>(buildShelfApiPath('/layers'), {
    method: 'GET'
  });
  if (!Array.isArray(data)) {
    throw new Error('Failed to fetch layers: invalid response format');
  }

  const unique = new Set<string>();
  for (const item of data) {
    const normalized = normalizeLayerValue(item);
    if (normalized) {
      unique.add(normalized);
    }
  }

  return Array.from(unique).sort((a, b) => a.localeCompare(b));
}

export async function createLayer(layerPath: string): Promise<void> {
  const normalized = normalizeLayerPath(layerPath);
  if (!normalized) {
    throw new Error('Layer path cannot be empty');
  }

  const encodedPath = encodeLayerPath(normalized);

  if (isMockApiMode()) {
    addMockLayer(normalized);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/layers/${encodedPath}`), {
      method: 'POST'
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      throw new LayerHttpError('Layer path cannot be empty');
    }

    if (err instanceof ApiError && err.status === 500) {
      throw new LayerHttpError('Failed to create layer');
    }

    const message = err instanceof Error ? err.message : 'Failed to create layer';
    throw new LayerHttpError(message || 'Failed to create layer');
  }
}

export async function renameLayer(layerPath: string, nextName: string): Promise<void> {
  const normalized = normalizeLayerPath(layerPath);
  const name = nextName.trim();
  if (!normalized || normalized === '/' || !name || name.includes('/')) {
    throw new LayerHttpError('Invalid layer name');
  }

  const parentSegments = layersFromPath(normalized).slice(0, -1);
  const nextPath = [...parentSegments, name].join('/');

  if (isMockApiMode()) {
    if (mockLayers.has(nextPath)) {
      throw new LayerHttpError('Layer already exists');
    }
    const currentLayers = getMockLayers();
    for (const layer of currentLayers) {
      if (layer === normalized || layer.startsWith(`${normalized}/`)) {
        mockLayers.delete(layer);
        mockLayers.add(replaceLayerPrefix(layer, normalized, nextPath));
      }
    }
    syncMockBooksLayerPrefix(normalized, nextPath);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/layers/${encodeLayerPath(normalized)}`), {
      method: 'PATCH',
      body: JSON.stringify({ name })
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      throw new LayerHttpError('Invalid layer name');
    }
    if (err instanceof ApiError && err.status === 409) {
      throw new LayerHttpError('Failed to rename layer');
    }
    const message = err instanceof Error ? err.message : 'Failed to rename layer';
    throw new LayerHttpError(message || 'Failed to rename layer');
  }
}

export async function moveLayer(layerPath: string, targetLayerPath: string): Promise<void> {
  const normalized = normalizeLayerPath(layerPath);
  const target = normalizeLayerPath(targetLayerPath);
  if (!normalized || normalized === '/') {
    throw new LayerHttpError('Invalid layer path');
  }
  if (target !== '' && target !== '/' && (target === normalized || target.startsWith(`${normalized}/`))) {
    throw new LayerHttpError('Cannot move a layer under itself.');
  }

  const sourceSegments = layersFromPath(normalized);
  const layerName = sourceSegments[sourceSegments.length - 1] ?? '';
  const destination = [...layersFromPath(target), layerName].join('/');

  if (isMockApiMode()) {
    if (!mockLayers.has(target || '/')) {
      throw new LayerHttpError('Target layer does not exist');
    }
    if (mockLayers.has(destination)) {
      throw new LayerHttpError('Target layer already contains a layer with this name');
    }
    const currentLayers = getMockLayers();
    for (const layer of currentLayers) {
      if (layer === normalized || layer.startsWith(`${normalized}/`)) {
        mockLayers.delete(layer);
        mockLayers.add(replaceLayerPrefix(layer, normalized, destination));
      }
    }
    syncMockBooksLayerPrefix(normalized, destination);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath('/layer-moves'), {
      method: 'POST',
      body: JSON.stringify({ layer: layersFromPath(normalized), target_layer: layersFromPath(target) })
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      throw new LayerHttpError('Invalid layer path');
    }
    if (err instanceof ApiError && err.status === 409) {
      throw new LayerHttpError('Failed to move layer');
    }
    const message = err instanceof Error ? err.message : 'Failed to move layer';
    throw new LayerHttpError(message || 'Failed to move layer');
  }
}

export async function deleteLayer(layerPath: string): Promise<void> {
  const normalized = normalizeLayerPath(layerPath);
  if (!normalized || normalized === '/') {
    throw new LayerHttpError('Cannot delete this layer because it is not empty.');
  }

  const encodedPath = encodeLayerPath(normalized);

  if (isMockApiMode()) {
    const hasBooks = mockBooks.some((book) => pathFromLayers(book.layers) === normalized);
    const hasChildren = getMockLayers().some((path) => path !== normalized && path.startsWith(`${normalized}/`));

    if (hasBooks || hasChildren) {
      throw new LayerHttpError('Cannot delete this layer because it is not empty.');
    }

    deleteMockLayer(normalized);
    await delay(undefined);
    return;
  }

  try {
    await fetchJson<void>(buildShelfApiPath(`/layers/${encodedPath}`), {
      method: 'DELETE'
    });
  } catch (err) {
    if (err instanceof ApiError && (err.status === 400 || err.status === 409)) {
      throw new LayerHttpError('Cannot delete this layer because it is not empty.');
    }

    const message = err instanceof Error ? err.message : 'Failed to delete layer';
    throw new LayerHttpError(message || 'Failed to delete layer');
  }
}
