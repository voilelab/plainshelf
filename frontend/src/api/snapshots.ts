import { fetchJson, fetchText, isMockApiMode } from './client';
import type { SnapshotMeta } from '../types/snapsnot';

interface SnapshotStoreItem {
  meta: SnapshotMeta;
  content: string;
}

const mockSnapshots: Record<string, SnapshotStoreItem[]> = {};

function countLines(value: string): number {
  return value.length === 0 ? 0 : value.split(/\r\n|\r|\n/).length;
}

function buildSnapshotMeta(id: string, createdAt: string, content: string): SnapshotMeta {
  return {
    id,
    created_at: createdAt,
    comment: 'Mock snapshot',
    md5_hash: hashText(content),
    line_count: countLines(content),
    char_count: content.length
  };
}

function normalizeSnapshotMeta(raw: unknown): SnapshotMeta {
  const record = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {};
  const meta: SnapshotMeta = {
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
    meta.split_config = record.split_config as SnapshotMeta['split_config'];
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

function ensureMockSnapshots(bookId: string): SnapshotStoreItem[] {
  if (mockSnapshots[bookId]) {
    return mockSnapshots[bookId];
  }

  const now = new Date();
  const firstId = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-090000`;
  const secondId = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-120000`;
  const firstContent = `# Snapshot ${firstId}\n\nBook ${bookId} sample content.`;
  const secondContent = `# Snapshot ${secondId}\n\nSecond snapshot for ${bookId}.`;

  mockSnapshots[bookId] = [
    {
      meta: buildSnapshotMeta(secondId, now.toISOString(), secondContent),
      content: secondContent
    },
    {
      meta: buildSnapshotMeta(firstId, new Date(now.getTime() - 30 * 60 * 1000).toISOString(), firstContent),
      content: firstContent
    }
  ];

  return mockSnapshots[bookId];
}

export async function listSnapshots(bookId: string): Promise<SnapshotMeta[]> {
  if (isMockApiMode()) {
    return ensureMockSnapshots(bookId).map((item) => ({ ...item.meta }));
  }

  const data = await fetchJson<unknown>(`/api/books/${encodeURIComponent(bookId)}/snapshots`);
  if (Array.isArray(data)) {
    return data.map(normalizeSnapshotMeta);
  }

  if (data && typeof data === 'object') {
    const record = data as Record<string, unknown>;
    if (Array.isArray(record.snapshots)) {
      return record.snapshots.map(normalizeSnapshotMeta);
    }
  }

  return [];
}

export async function getSnapshot(bookId: string, snapshotId: string): Promise<SnapshotMeta> {
  if (isMockApiMode()) {
    const item = ensureMockSnapshots(bookId).find((snapshot) => snapshot.meta.id === snapshotId);
    if (!item) {
      throw new Error('Snapshot not found');
    }
    return { ...item.meta };
  }

  const data = await fetchJson<unknown>(
    `/api/books/${encodeURIComponent(bookId)}/snapshots/${encodeURIComponent(snapshotId)}`
  );
  return normalizeSnapshotMeta(data);
}

export async function getSnapshotContent(bookId: string, snapshotId: string): Promise<string> {
  if (isMockApiMode()) {
    const item = ensureMockSnapshots(bookId).find((snapshot) => snapshot.meta.id === snapshotId);
    if (!item) {
      throw new Error('Snapshot not found');
    }
    return item.content;
  }

  return await fetchText(
    `/api/books/${encodeURIComponent(bookId)}/snapshots/${encodeURIComponent(snapshotId)}/content`
  );
}

export async function updateSnapshotContent(bookId: string, snapshotId: string, content: string): Promise<void> {
  if (isMockApiMode()) {
    const snapshots = ensureMockSnapshots(bookId);
    const item = snapshots.find((snapshot) => snapshot.meta.id === snapshotId);
    if (!item) {
      throw new Error('Snapshot not found');
    }
    item.content = content;
    item.meta = buildSnapshotMeta(item.meta.id, item.meta.created_at, content);
    return;
  }

  await fetchText(
    `/api/books/${encodeURIComponent(bookId)}/snapshots/${encodeURIComponent(snapshotId)}/content`,
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
