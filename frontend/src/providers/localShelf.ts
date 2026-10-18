import { registerPlugin } from '@capacitor/core';
import type { NsfwMarks } from '@/types/book';
import type { ShelfSourceCheck, ShelfSourceKind } from '@/utils/shelfSourceCheck';

/**
 * The Android app's local shelves: the embedded Go server (mobile/), reached
 * without a socket.
 *
 * Reads under LOCAL_API_BASE are plain same-origin requests that the native
 * WebViewClient answers from Go, so `<img src>` and streamed book text work
 * unchanged. The WebView hands it no request body, so every other method goes
 * through the plugin, with the body staged in chunks into a native cache file.
 */

/** Where api/client.ts points while a local shelf is active: the WebView's own origin. */
export const LOCAL_API_BASE = 'https://localhost/__plainshelf_local';

/** One shelf as the Go side lists it. */
interface LocalShelfInfo {
  id: string;
  name: string;
  /** RFC 3339; absent until the first export. */
  last_export?: string;
}

interface LocalShelfResponse {
  status: number;
  headers: Record<string, string>;
  /** Base64. */
  body: string;
}

interface LocalShelfPlugin {
  listShelves(): Promise<{ shelves: LocalShelfInfo[] }>;
  createShelf(options: { name: string }): Promise<{ id: string }>;
  renameShelf(options: { id: string; name: string }): Promise<void>;
  /** Deletes the shelf's directory with every book in it. */
  removeShelf(options: { id: string }): Promise<void>;
  /**
   * Writes the shelf as a zip, then lets the user pick where it goes. Records
   * last_export only once the zip is there; a cancel leaves no file behind.
   */
  exportShelf(options: { id: string; fileName: string }): Promise<{ cancelled: boolean }>;
  /** Lets the user pick a shelf zip and makes a new shelf from it, named after the file. */
  importShelf(options: { fallbackName: string }): Promise<{ cancelled: boolean; id?: string; name?: string }>;
  /**
   * Opens a pCloud shelf entry in the embedded server as the virtual shelf
   * `id`. The plugin reads the entry's token from secure storage itself, so it
   * never crosses the bridge. `showNsfw` is the device's adult-content choice.
   */
  openPCloudShelf(options: {
    id: string;
    entryId: string;
    name: string;
    host: string;
    folder: string;
    scope: string;
    showNsfw: boolean;
  }): Promise<void>;
  /**
   * Opens a plainshelf or webdav entry in the embedded server as the virtual
   * shelf `id`. Go resolves the entry's secret from secure storage itself
   * (`host:<entryId>`), so it never crosses the bridge.
   */
  openSourceShelf(options: {
    id: string;
    entryId: string;
    name: string;
    source: ShelfSourceKind;
    url: string;
    username: string;
    showNsfw: boolean;
  }): Promise<void>;
  /**
   * Probes a source during setup, before an entry stores the secret, so the
   * form passes the one it holds.
   */
  /** Removes Go offline copies for an entry; never used for switching entries. */
  removeVirtualShelfOffline(options: { scope: string; source?: ShelfSourceKind; url?: string; username?: string }): Promise<void>;
  checkSource(options: {
    source: ShelfSourceKind;
    url: string;
    username: string;
    secret: string;
  }): Promise<ShelfSourceCheck>;
  /** Changes the adult-content choice of a shelf openPCloudShelf or openSourceShelf opened. */
  setShelfShowNsfw(options: { id: string; show: boolean }): Promise<void>;
  /** Every book's marks on that shelf, whatever the device shows. */
  nsfwMarks(options: { id: string }): Promise<{ marks: Record<string, NsfwMarks> }>;
  /**
   * Setup calls, made before an entry stores the token, so the form passes the
   * one it holds. Go owns the shelf layout: whether a folder is a shelf, what a
   * book is.
   */
  checkPCloudShelf(options: {
    host: string;
    token: string;
    folder: string;
  }): Promise<{ isShelf: boolean; books: number }>;
  /** One picker level: subfolders without book packages, and whether it is a shelf root. */
  listPCloudFolder(options: {
    host: string;
    token: string;
    folderId: number;
  }): Promise<{ folders: Array<{ name: string; folderid: number }>; isShelf: boolean }>;
  beginBody(): Promise<{ bodyId: string }>;
  appendBody(options: { bodyId: string; data: string }): Promise<void>;
  /** Deletes a staged body that will not be sent. */
  discardBody(options: { bodyId: string }): Promise<void>;
  request(options: {
    method: string;
    path: string;
    headers: Record<string, string>;
    bodyId?: string;
  }): Promise<LocalShelfResponse>;
}

// No web implementation: the browser preview has no embedded server.
export const LocalShelf = registerPlugin<LocalShelfPlugin>('PlainShelfLocalShelf');

/** A backup's suggested file name: `plainshelf-<name>-<yyyymmdd>.zip`. */
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

/**
 * Plugin errors reach here as message text only, so the Go sentinels
 * (mobile.ErrBusy, internal/shelfzip) are matched by their wording.
 */
const LOCAL_SHELF_ERROR_KEYS: Array<[string, string]> = [
  ['background tasks are still running', 'mobileShelves.errorBusy'],
  ['not a PlainShelf shelf', 'mobileShelves.errorNotShelf'],
  ['zip entry escapes the shelf', 'mobileShelves.errorUnsafePath'],
  ['zip unpacks to more than', 'mobileShelves.errorTooLarge'],
  ['not a valid zip file', 'mobileShelves.errorNotZip']
];

/** The i18n key explaining a failed export or import, or null when unrecognised. */
export function localShelfErrorKey(err: unknown): string | null {
  const message = err instanceof Error ? err.message : String(err);
  return LOCAL_SHELF_ERROR_KEYS.find(([needle]) => message.includes(needle))?.[1] ?? null;
}

/** Bytes per appendBody call; each crosses the bridge as base64. */
const BODY_CHUNK_BYTES = 1 << 20;

function toBase64(bytes: Uint8Array): string {
  let binary = '';
  // String.fromCharCode takes its arguments on the stack; keep calls small.
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

// A Response refuses a body, even an empty one, for these.
const NULL_BODY_STATUSES = new Set([101, 204, 205, 304]);

/**
 * The fetch api/client.ts uses while a local shelf is active. Anything outside
 * LOCAL_API_BASE, and every GET or HEAD, is an ordinary fetch.
 */
export async function localShelfFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const request = new Request(input, init);
  const method = request.method.toUpperCase();
  if (!request.url.startsWith(`${LOCAL_API_BASE}/`) || method === 'GET' || method === 'HEAD') {
    return fetch(input, init);
  }

  // Request serializes FormData itself, boundary included in Content-Type.
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
      // request() deletes a body it was given; one that never gets there is ours.
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
