import { ApiError, buildShelfApiPath, fetchJson, isMockApiMode } from './client';
import { LayerHttpError } from './layerErrors';
import { delay } from './mocks/latency';
import {
  addMockLayer,
  deleteMockLayer,
  getMockLayers,
  moveMockLayer,
  renameMockLayer
} from './mocks/layers';
import { normalizeLayerPath } from '@/utils/layers';

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

function encodeLayerPath(path: string): string {
  return path
    .split('/')
    .filter((segment) => segment.length > 0)
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

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
      // The server names the reason it refused the name - an empty path, or a
      // hidden/system directory name its scanner would skip - and the user
      // cannot guess between them, so pass the message through instead of
      // replacing it with one fixed guess.
      throw new LayerHttpError(err.message || 'Layer path cannot be empty');
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
    renameMockLayer(normalized, nextPath);
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
    moveMockLayer(normalized, target, destination);
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
