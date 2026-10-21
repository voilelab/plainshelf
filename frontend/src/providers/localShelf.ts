import { registerPlugin } from '@capacitor/core';
import type { NsfwMarks } from '@/types/book';
import type { ShelfSourceCheck, ShelfSourceKind } from '@/utils/shelfSourceCheck';

// Android WebResourceRequest exposes the method, URL and headers but no request body.

export const LOCAL_API_BASE = 'https://localhost/__plainshelf_local';

interface LocalShelfInfo {
  id: string;
  name: string;
  last_export?: string;
}

interface LocalShelfResponse {
  status: number;
  headers: Record<string, string>;
  body: string;
}

interface LocalShelfPlugin {
  parseLANShareQR(options: { text: string }): Promise<{
    token: string;
    shelves: Array<{ id: string; name: string; url: string }>;
  }>;
  listShelves(): Promise<{ shelves: LocalShelfInfo[] }>;
  createShelf(options: { name: string }): Promise<{ id: string }>;
  renameShelf(options: { id: string; name: string }): Promise<void>;
  removeShelf(options: { id: string }): Promise<void>;
  exportShelf(options: { id: string; fileName: string }): Promise<{ cancelled: boolean }>;
  importShelf(options: { fallbackName: string }): Promise<{ cancelled: boolean; id?: string; name?: string }>;
  openPCloudShelf(options: {
    id: string;
    entryId: string;
    name: string;
    host: string;
    folder: string;
    scope: string;
    showNsfw: boolean;
  }): Promise<void>;
  openSourceShelf(options: {
    id: string;
    entryId: string;
    name: string;
    source: ShelfSourceKind;
    url: string;
    username: string;
    showNsfw: boolean;
  }): Promise<void>;
  sourceShelfOfflineScope(options: { source: ShelfSourceKind; url: string; username: string }): Promise<{ scope: string }>;
  removeVirtualShelfOffline(options: { scope: string; source?: ShelfSourceKind; url?: string; username?: string }): Promise<void>;
  checkSource(options: {
    source: ShelfSourceKind;
    url: string;
    username: string;
    secret: string;
  }): Promise<ShelfSourceCheck>;
  setShelfShowNsfw(options: { id: string; show: boolean }): Promise<void>;
  nsfwMarks(options: { id: string }): Promise<{ marks: Record<string, NsfwMarks> }>;
  checkPCloudShelf(options: {
    host: string;
    token: string;
    folder: string;
  }): Promise<{ isShelf: boolean; books: number }>;
  listPCloudFolder(options: {
    host: string;
    token: string;
    folderId: number;
  }): Promise<{ folders: Array<{ name: string; folderid: number }>; isShelf: boolean }>;
  beginBody(): Promise<{ bodyId: string }>;
  appendBody(options: { bodyId: string; data: string }): Promise<void>;
  discardBody(options: { bodyId: string }): Promise<void>;
  request(options: {
    method: string;
    path: string;
    headers: Record<string, string>;
    bodyId?: string;
  }): Promise<LocalShelfResponse>;
}

export const LocalShelf = registerPlugin<LocalShelfPlugin>('PlainShelfLocalShelf');

export function localShelfExportFileName(name: string, now: Date = new Date()): string {
  const safe = name
    .trim()
    .replace(/[\\/:*?"<>|\u0000-\u001f]+/g, '-')
    .replace(/\s+/g, ' ')
    .replace(/^-+|-+$/g, '');
  const pad = (n: number) => String(n).padStart(2, '0');
  const date = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}`;
  return safe ? `plainshelf-${safe}-${date}.zip` : `plainshelf-${date}.zip`;
}

const LOCAL_SHELF_ERROR_KEYS: Array<[string, string]> = [
  ['background tasks are still running', 'mobileShelves.errorBusy'],
  ['not a PlainShelf shelf', 'mobileShelves.errorNotShelf'],
  ['zip entry escapes the shelf', 'mobileShelves.errorUnsafePath'],
  ['zip unpacks to more than', 'mobileShelves.errorTooLarge'],
  ['not a valid zip file', 'mobileShelves.errorNotZip']
];

export function localShelfErrorKey(err: unknown): string | null {
  const message = err instanceof Error ? err.message : String(err);
  return LOCAL_SHELF_ERROR_KEYS.find(([needle]) => message.includes(needle))?.[1] ?? null;
}

const BODY_CHUNK_BYTES = 1 << 20;

function toBase64(bytes: Uint8Array): string {
  let binary = '';
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  }
  return btoa(binary);
}

function fromBase64(data: string): Uint8Array<ArrayBuffer> {
  const binary = atob(data);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

const NULL_BODY_STATUSES = new Set([101, 204, 205, 304]);

export async function localShelfFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const request = new Request(input, init);
  const method = request.method.toUpperCase();
  if (!request.url.startsWith(`${LOCAL_API_BASE}/`) || method === 'GET' || method === 'HEAD') {
    return fetch(input, init);
  }

  const body = new Uint8Array(await request.arrayBuffer());
  let bodyId: string | undefined;
  if (body.length > 0) {
    ({ bodyId } = await LocalShelf.beginBody());
    try {
      for (let i = 0; i < body.length; i += BODY_CHUNK_BYTES) {
        request.signal.throwIfAborted();
        await LocalShelf.appendBody({ bodyId, data: toBase64(body.subarray(i, i + BODY_CHUNK_BYTES)) });
      }
      request.signal.throwIfAborted();
    } catch (err) {
      await LocalShelf.discardBody({ bodyId }).catch(() => undefined);
      throw err;
    }
  }
  request.signal.throwIfAborted();

  const headers: Record<string, string> = {};
  request.headers.forEach((value, key) => {
    headers[key] = value;
  });
  const res = await LocalShelf.request({
    method,
    path: request.url.slice(LOCAL_API_BASE.length),
    headers,
    bodyId
  });
  return new Response(NULL_BODY_STATUSES.has(res.status) ? null : fromBase64(res.body), {
    status: res.status,
    headers: res.headers
  });
}
