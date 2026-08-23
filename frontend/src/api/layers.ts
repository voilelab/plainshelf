import type { BookTransferMode } from './books';
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
import { mockTransferLayer } from './mocks/books';
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

/**
 * Lists the layers of a shelf. Defaults to the active shelf; pass `shelfID` to
 * read another shelf's layers, which is what the cross-shelf transfer picker
 * needs — its destination layers belong to the target shelf, not the active one.
 */
export async function getLayers(shelfID?: string): Promise<string[]> {
  if (isMockApiMode()) {
    return delay(getMockLayers());
  }

  const data: unknown = await fetchJson<unknown>(buildShelfApiPath('/layers', shelfID), {
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

/** Which cross-shelf conflict a layer transfer was refused for. */
export type LayerTransferConflictKind = 'target_layer_conflict' | 'book_id_conflict';

/**
 * Thrown by {@link transferLayer} when the server refuses the transfer up front
 * with a 409 conflict body — either the target shelf already holds a folder with
 * this name, or (for a move) it already holds books with these IDs. A distinct
 * type, not a bare Error, so the modal can name which conflict it is and list the
 * colliding book IDs rather than showing one opaque message. The server's own
 * message is kept as `message` so the UI can fall back to it.
 */
export class LayerTransferConflictError extends Error {
  constructor(
    readonly kind: LayerTransferConflictKind,
    message: string,
    readonly conflictingBookIDs: string[] = []
  ) {
    super(message);
    this.name = 'LayerTransferConflictError';
  }
}

interface LayerTransferConflictBody {
  error?: string;
  message?: string;
  conflicting_book_ids?: string[];
  taskchain_id?: string;
}

/**
 * Copies or moves a whole layer (folder) — every book and sub-folder beneath it —
 * from the active shelf to `targetShelfID`, returning the id of the background
 * task chain to poll. The work is asynchronous because a folder can hold hundreds
 * of megabytes and either shelf may be a network mount, so the request must not
 * block on it.
 *
 * `sourceLayer` and `targetLayer` are '/'-joined paths. `targetLayer` is the full
 * destination path the folder lands at on the target shelf (it keeps its own name
 * when the caller nests it under a chosen parent); the server refuses a root
 * target. When the same transfer is already running the server answers 409 with
 * that chain's id, returned here so the caller attaches to the in-flight work.
 * The two pre-flight conflicts (target folder exists, or a move would overwrite
 * book IDs) are surfaced as {@link LayerTransferConflictError}; any other refusal
 * (a read-only target, an invalid path) propagates with the server's message.
 */
export async function transferLayer(
  sourceLayer: string,
  targetShelfID: string,
  targetLayer: string,
  mode: BookTransferMode
): Promise<string> {
  if (isMockApiMode()) {
    return mockTransferLayer(sourceLayer, targetShelfID, targetLayer, mode);
  }

  const source = layersFromPath(sourceLayer);
  const target = layersFromPath(targetLayer);

  try {
    const res = await fetchJson<{ taskchain_id: string }>(
      buildShelfApiPath('/layer-transfers'),
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode, source_layer: source, target_shelf: targetShelfID, target_layer: target })
      }
    );
    return res.taskchain_id;
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const conflict = parseLayerTransferConflict(err.message);
      // The already-running case answers 409 with the chain id, not a conflict
      // body; attach to it rather than failing.
      if (conflict?.taskchain_id) {
        return conflict.taskchain_id;
      }
      if (conflict?.error === 'target_layer_conflict' || conflict?.error === 'book_id_conflict') {
        throw new LayerTransferConflictError(
          conflict.error,
          conflict.message || err.message,
          conflict.conflicting_book_ids ?? []
        );
      }
    }
    throw err;
  }
}

function parseLayerTransferConflict(body: string): LayerTransferConflictBody | null {
  try {
    const parsed = JSON.parse(body) as LayerTransferConflictBody;
    return parsed && typeof parsed === 'object' ? parsed : null;
  } catch {
    // A plain-text 409 (a read-only shelf) is not a conflict body; let the caller
    // propagate the original ApiError so its message reaches the user.
    return null;
  }
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
