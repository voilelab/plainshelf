import { fetchJson, fetchText, isMockApiMode } from './client';
import type { SourceMeta as SourceMeta } from '../types/snapsnot';

interface SourceStoreItem {
  meta: SourceMeta;
  content: string;
}

const mockSource: Record<string, SourceStoreItem[]> = {};

function countLines(value: string): number {
  return value.length === 0 ? 0 : value.split(/\r\n|\r|\n/).length;
}

function buildSourceMeta(id: string, createdAt: string, content: string): SourceMeta {
  return {
    id,
    created_at: createdAt,
    comment: 'Mock source',
    md5_hash: hashText(content),
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

  if (typeof record.line_count === 'number' && Number.isFinite(record.line_count)) {
    meta.line_count = Math.trunc(record.line_count);
  }

  if (typeof record.char_count === 'number' && Number.isFinite(record.char_count)) {
    meta.char_count = Math.trunc(record.char_count);
  }

  if (record.split_config && typeof record.split_config === 'object') {
    meta.split_config = record.split_config as SourceMeta['split_config'];
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

  const data = await fetchJson<unknown>(`/api/books/${encodeURIComponent(bookId)}/sources`);
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
    `/api/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}`
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
    `/api/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/content`
  );
}

export async function updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
  if (isMockApiMode()) {
    const sources = ensureMockSource(bookId);
    const item = sources.find((source) => source.meta.id === sourceId);
    if (!item) {
      throw new Error('Source not found');
    }
    item.content = content;
    item.meta = buildSourceMeta(item.meta.id, item.meta.created_at, content);
    return;
  }

  await fetchText(
    `/api/books/${encodeURIComponent(bookId)}/sources/${encodeURIComponent(sourceId)}/content`,
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
