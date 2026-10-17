import { registerPlugin } from '@capacitor/core';

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
  beginBody(): Promise<{ bodyId: string }>;
  appendBody(options: { bodyId: string; data: string }): Promise<void>;
  request(options: {
    method: string;
    path: string;
    headers: Record<string, string>;
    bodyId?: string;
  }): Promise<LocalShelfResponse>;
}

// No web implementation: the browser preview has no embedded server.
export const LocalShelf = registerPlugin<LocalShelfPlugin>('PlainShelfLocalShelf');

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
    for (let i = 0; i < body.length; i += BODY_CHUNK_BYTES) {
      request.signal.throwIfAborted();
      await LocalShelf.appendBody({ bodyId, data: toBase64(body.subarray(i, i + BODY_CHUNK_BYTES)) });
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
