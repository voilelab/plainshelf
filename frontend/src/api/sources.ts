import { buildShelfApiPath, fetchBlob, fetchJson, fetchText, isMockApiMode } from './client';
import type { CreateSourceOptions, SourceMeta } from '@/types/source';

interface SourceStoreItem {
  meta: SourceMeta;
  content: string;
}

const mockSource: Record<string, SourceStoreItem[]> = {};
const SOURCE_UPLOAD_TIMEOUT_MS = 300_000;

function countLines(value: string): number {
  return value.length === 0 ? 0 : value.split(/\r\n|\r|\n/).length;
}

function buildSourceMeta(id: string, createdAt: string, content: string, format: 'txt' | 'md' = 'txt'): SourceMeta {
  return {
    schema_version: 1,
    id,
    created_at: createdAt,
    comment: 'Mock source',
    md5_hash: hashText(content),
    format,
    line_count: countLines(content),
    char_count: content.length
  };
}

function normalizeSourceMeta(raw: unknown): SourceMeta {
  const record = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {};
  const meta: SourceMeta = {
    id: typeof record.id === 'string' ? record.id : '',
    created_at: typeof record.created_at === 'string' ? record.created_at : '',
    comment: typeof record.comment === 'string' ? record.comment : '',
    md5_hash: typeof record.md5_hash === 'string' ? record.md5_hash : ''
  };

  if (typeof record.schema_version === 'number' && Number.isFinite(record.schema_version)) {
    meta.schema_version = Math.trunc(record.schema_version);
  }

  if (record.format === 'txt' || record.format === 'md') {
    meta.format = record.format;
  }

  if (typeof record.line_count === 'number' && Number.isFinite(record.line_count)) {
    meta.line_count = Math.trunc(record.line_count);
  }

  if (typeof record.char_count === 'number' && Number.isFinite(record.char_count)) {
    meta.char_count = Math.trunc(record.char_count);
  }

  return meta;
}

function hashText(value: string): string {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash +=
      (hash << 1) +
      (hash << 4) +
      (hash << 7) +
      (hash << 8) +
      (hash << 24);
  }

  return (hash >>> 0).toString(16).padStart(8, '0');
}

function ensureMockSource(bookId: string): SourceStoreItem[] {
  if (mockSource[bookId]) {
    return mockSource[bookId];
  }

  const now = new Date();
  const firstId = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-090000`;
  const secondId = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-120000`;
  const firstContent = `# Source ${firstId}\n\nBook ${bookId} sample content.`;
  const secondContent = `# Source ${secondId}\n\nSecond source for ${bookId}.`;

  mockSource[bookId] = [
    {
      meta: buildSourceMeta(secondId, now.toISOString(), secondContent),
      content: secondContent
    },
    {
      meta: buildSourceMeta(firstId, new Date(now.getTime() - 30 * 60 * 1000).toISOString(), firstContent),
      content: firstContent
    }
  ];

  return mockSource[bookId];
}

export async function listSource(bookId: string): Promise<SourceMeta[]> {
  if (isMockApiMode()) {
    return ensureMockSource(bookId).map((item) => ({ ...item.meta }));
  }

  const data = await fetchJson<unknown>(buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources`));
  if (Array.isArray(data)) {
    return data.map(normalizeSourceMeta);
  }

  return [];
}

export async function getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
  if (isMockApiMode()) {
    const item = ensureMockSource(bookId).find((source) => source.meta.id === sourceId);
    if (!item) {
      throw new Error('Source not found');
    }
    return { ...item.meta };
  }

  const data = await fetchJson<unknown>(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}`)
  );
  return normalizeSourceMeta(data);
}

export async function getSourceContent(bookId: string, sourceId: string): Promise<string> {
  if (isMockApiMode()) {
    const item = ensureMockSource(bookId).find((source) => source.meta.id === sourceId);
    if (!item) {
      throw new Error('Source not found');
    }
    return item.content;
  }

  return await fetchText(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/content`)
  );
}

/**
 * Fetches one of a source's illustrations.
 *
 * This goes through `fetchBlob` rather than being handed to an `<img src>`,
 * because the request has to carry the API token: a plain image request would
 * be issued by the browser without it and fail whenever the server sets
 * `protect_read`.
 */
export async function getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob> {
  if (isMockApiMode()) {
    throw new Error('Source assets are not served in mock API mode');
  }

  return await fetchBlob(
    buildShelfApiPath(
      `/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/assets/${encodeURIComponent(name)}`
    )
  );
}

/**
 * Fetches a source's illustrations as one zip, so a download pays a single
 * request instead of one per figure. `names` picks which files to pack (empty =
 * the whole `assets/` directory); an absent name is packed as no entry.
 *
 * Goes through `fetchBlob`, like getSourceAsset, so the request carries the API
 * token a plain browser request would omit under `protect_read`. Returns the
 * raw archive so the caller can unzip one entry at a time.
 */
export async function getSourceAssetsBundle(
  bookId: string,
  sourceId: string,
  names: string[]
): Promise<Blob> {
  if (isMockApiMode()) {
    throw new Error('Source assets are not served in mock API mode');
  }

  const path = buildShelfApiPath(
    `/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/assets.zip`
  );
  const query = names.map((name) => `name=${encodeURIComponent(name)}`).join('&');
  return await fetchBlob(query ? `${path}?${query}` : path);
}

export async function createSource(bookId: string, options?: CreateSourceOptions): Promise<SourceMeta> {
  if (isMockApiMode()) {
    const sources = ensureMockSource(bookId);
    const now = new Date();
    const id = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}${String(now.getSeconds()).padStart(2, '0')}-${String(now.getMilliseconds()).padStart(3, '0')}-${Math.random().toString(36).slice(2, 7)}`;
    const content = options?.content ?? '';
    const format = options?.format ?? 'txt';
    const newItem: SourceStoreItem = {
      meta: {
        ...buildSourceMeta(id, now.toISOString(), content, format),
        comment: options?.comment ?? ''
      },
      content
    };
    sources.unshift(newItem);
    return { ...newItem.meta };
  }

  // Wails routes fetches through the WebView's custom scheme handler. WebKit
  // can truncate a generated multipart body there, especially once a derived
  // source crosses the old in-memory form-field threshold; Go then reports
  // `multipart: NextPart: EOF`. JSON is serialized as one ordinary request
  // body and avoids that streaming path. The server still accepts multipart
  // for older clients.
  const body = options ? JSON.stringify({
    content: options.content ?? '',
    format: options.format ?? 'txt',
    comment: options.comment ?? '',
    set_current: options.setCurrent ?? false
  }) : undefined;
  const data = await fetchJson<unknown>(buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources`), {
    method: 'POST',
    ...(options ? { headers: { 'Content-Type': 'application/json' } } : {}),
    body
  }, {
    timeoutMs: SOURCE_UPLOAD_TIMEOUT_MS
  });
  return normalizeSourceMeta(data);
}

export async function deleteSource(bookId: string, sourceId: string): Promise<void> {
  if (isMockApiMode()) {
    const sources = ensureMockSource(bookId);
    const index = sources.findIndex((source) => source.meta.id === sourceId);
    if (index === -1) {
      throw new Error('Source not found');
    }
    sources.splice(index, 1);
    return;
  }

  await fetchJson<void>(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}`),
    { method: 'DELETE' }
  );
}

/**
 * Clears the source's import note. The note records where the text came from,
 * so the API only removes one — there is no rewrite to pair with this.
 */
export async function deleteSourceComment(bookId: string, sourceId: string): Promise<void> {
  if (isMockApiMode()) {
    const item = ensureMockSource(bookId).find((source) => source.meta.id === sourceId);
    if (!item) {
      throw new Error('Source not found');
    }
    item.meta = { ...item.meta, comment: '' };
    return;
  }

  await fetchJson<void>(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/comment`),
    { method: 'DELETE' }
  );
}

export async function setCurrentSource(bookId: string, sourceId: string): Promise<void> {
  if (isMockApiMode()) {
    const sources = ensureMockSource(bookId);
    const found = sources.some((source) => source.meta.id === sourceId);
    if (!found) {
      throw new Error('Source not found');
    }
    return;
  }

  await fetchJson<void>(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/current`),
    { method: 'PUT' }
  );
}

export async function refreshSourceMeta(bookId: string, sourceId: string): Promise<SourceMeta> {
  if (isMockApiMode()) {
    const item = ensureMockSource(bookId).find((source) => source.meta.id === sourceId);
    if (!item) {
      throw new Error('Source not found');
    }
    item.meta = {
      ...buildSourceMeta(item.meta.id, item.meta.created_at, item.content, item.meta.format === 'md' ? 'md' : 'txt'),
      comment: item.meta.comment
    };
    return { ...item.meta };
  }

  const data = await fetchJson<unknown>(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/refresh`),
    { method: 'POST' }
  );
  return normalizeSourceMeta(data);
}

export async function updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
  if (isMockApiMode()) {
    const sources = ensureMockSource(bookId);
    const item = sources.find((source) => source.meta.id === sourceId);
    if (!item) {
      throw new Error('Source not found');
    }
    item.content = content;
    item.meta = {
      ...buildSourceMeta(item.meta.id, item.meta.created_at, content, item.meta.format === 'md' ? 'md' : 'txt'),
      comment: item.meta.comment
    };
    return;
  }

  await fetchText(
    buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/content`),
    {
      method: 'PATCH',
      headers: {
        Accept: 'text/plain',
        'Content-Type': 'text/plain'
      },
      body: content
    }
  );
}
