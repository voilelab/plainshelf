import { fetchJson, fetchText, isMockApiMode } from './client';
import type { SnapshotMeta } from '../types/snapsnot';

interface SnapshotStoreItem {
  meta: SnapshotMeta;
  content: string;
}

const mockSnapshots: Record<string, SnapshotStoreItem[]> = {};

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
      meta: {
        id: secondId,
        created_at: now.toISOString(),
        comment: 'Mock snapshot',
        md5_hash: hashText(secondContent)
      },
      content: secondContent
    },
    {
      meta: {
        id: firstId,
        created_at: new Date(now.getTime() - 30 * 60 * 1000).toISOString(),
        comment: 'Mock snapshot',
        md5_hash: hashText(firstContent)
      },
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
    return data as SnapshotMeta[];
  }

  if (data && typeof data === 'object') {
    const record = data as Record<string, unknown>;
    if (Array.isArray(record.snapshots)) {
      return record.snapshots as SnapshotMeta[];
    }
  }

  return [];
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
    item.meta.md5_hash = hashText(content);
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
