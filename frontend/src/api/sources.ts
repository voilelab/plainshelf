import { buildShelfApiPath, fetchBlob, fetchJson, fetchText, isMockApiMode } from './client';
import type { CreateSourceOptions, SourceMeta } from '@/types/source';
import { normalizeSplitConfig } from '@/utils/splitConfig';

interface SourceStoreItem {
  meta: SourceMeta;
  content: string;
}

const mockSource: Record<string, SourceStoreItem[]> = {};
const MULTIPART_TEXT_CONTENT_LIMIT = 8 << 20;
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

  if (record.split_config && typeof record.split_config === 'object') {
    meta.split_config = normalizeSplitConfig(record.split_config);
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

  const body = options
    ? (() => {
        const form = new FormData();
        const sourceContent = options.content ?? '';
        // WKWebView can stall while streaming a programmatically-created Blob
        // in FormData. The client then aborts at its timeout and Go observes a
        // truncated multipart boundary (`NextPart: EOF`). Keep ordinary book
        // text as a multipart value; use a real File only when the value is
        // large enough that ParseMultipartForm should spill it to disk.
        if (new Blob([sourceContent]).size <= MULTIPART_TEXT_CONTENT_LIMIT) {
          form.append('content', sourceContent);
        } else {
          form.append('content', new File([sourceContent], 'source.txt', { type: 'text/plain;charset=utf-8' }));
        }
        form.append('format', options.format ?? 'txt');
        if (options.comment) {
          form.append('comment', options.comment);
        }
        if (options.setCurrent) {
          form.append('set_current', 'true');
        }
        return form;
      })()
    : undefined;
  const data = await fetchJson<unknown>(buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/sources`), {
    method: 'POST',
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
      comment: item.meta.comment,
      split_config: item.meta.split_config
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
      comment: item.meta.comment,
      split_config: item.meta.split_config
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
