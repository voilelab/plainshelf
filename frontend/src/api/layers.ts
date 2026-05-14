import { mockBooks } from './books';
import { ApiError, fetchJson, isMockApiMode } from './client';
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

class LayerHttpError extends Error {}

export async function getLayers(): Promise<string[]> {
  if (isMockApiMode()) {
    return delay(getMockLayers());
  }

  const data: unknown = await fetchJson<unknown>('/api/layers', {
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
    await fetchJson<void>(`/api/layers/${encodedPath}`, {
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
    await fetchJson<void>(`/api/layers/${encodedPath}`, {
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
