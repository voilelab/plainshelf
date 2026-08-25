import { isMobileRuntime } from '@/providers/runtime';

export class ApiError extends Error {
  status?: number;
  statusText?: string;
  url?: string;
  isTimeout: boolean;

  constructor(
    message: string,
    options?: {
      status?: number;
      statusText?: string;
      url?: string;
      cause?: unknown;
      isTimeout?: boolean;
    }
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = options?.status;
    this.statusText = options?.statusText;
    this.url = options?.url;
    this.isTimeout = options?.isTimeout ?? false;

    if (options?.cause !== undefined) {
      (this as Error & { cause?: unknown }).cause = options.cause;
    }
  }
}

type ApiMode = 'live' | 'mock';

declare global {
  interface Window {
    __PLAINSHELF_SECURITY__?: {
      token?: string;
      tokenHeader?: string;
    };
    __PLAINSHELF_READ_ONLY__?: boolean;
    plainshelf?: {
      getApiToken?: () => string | Promise<string>;
    };
  }
}

const RAW_API_BASE = String(import.meta.env.VITE_API_BASE ?? '').trim();
const API_BASE_NORMALIZED = RAW_API_BASE.replace(/\/+$/, '');
const USE_MOCK_OPT_IN = String(import.meta.env.VITE_USE_MOCK_API ?? '').toLowerCase() === 'true';
const ENV_API_TOKEN = String(import.meta.env.VITE_PLAINSHELF_TOKEN ?? '').trim();
const IS_DEV = import.meta.env.DEV;

if (USE_MOCK_OPT_IN && !IS_DEV) {
  throw new Error('VITE_USE_MOCK_API=true is development-only and cannot be enabled in production.');
}

const API_MODE: ApiMode = IS_DEV && USE_MOCK_OPT_IN ? 'mock' : 'live';

if (IS_DEV && API_MODE === 'mock') {
  console.info('[api] MOCK API mode enabled (VITE_USE_MOCK_API=true).');
}

// Build-time default. On native (Capacitor) builds there is no server to inject
// a base URL, so the mobile bootstrap can override this at runtime via
// setApiBase() once the user has entered their server address.
export const API_BASE = API_BASE_NORMALIZED;
let apiBase = API_BASE_NORMALIZED;

export function getApiBase(): string {
  return apiBase;
}

/**
 * The canonical form of a base URL.
 *
 * Exported because the stored value and the applied value have to agree
 * exactly: on the mobile shell the applied base is half of the key that scopes
 * device-local book data (providers/cacheScope.ts), so a caller deriving that
 * key from a saved shelf must normalize the same way this does — a trailing
 * slash left on one side and stripped on the other points at a different cache.
 */
export function normalizeApiBase(base: string): string {
  return String(base ?? '').trim().replace(/\/+$/, '');
}

export function setApiBase(base: string): void {
  apiBase = normalizeApiBase(base);
}

const SHELF_STORAGE_KEY = 'plainshelf.shelf';
let activeShelfID = '';

if (typeof window !== 'undefined') {
  const storedShelfID = window.localStorage.getItem(SHELF_STORAGE_KEY)?.trim();
  if (storedShelfID) {
    activeShelfID = storedShelfID;
  }
}

export function isMockApiMode(): boolean {
  return API_MODE === 'mock';
}

function assertApiMode(): void {
  if (API_MODE === 'mock' && !IS_DEV) {
    throw new Error('Mock API mode is only allowed in development.');
  }
}


// The mobile shell is a reading client and issues no writes at all: reading
// history, reading progress and reading stats are all stored on the device and
// never sent. Every mutation is rejected before it leaves the device.
function assertWritableRequest(init?: RequestInit, options?: FetchJsonOptions): void {
  const method = String(init?.method ?? 'GET').toUpperCase();
  if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) {
    return;
  }

  // A POST that only reads is still a read, and both gates below exist to stop
  // writes. The server draws the same exception for the same route; see
  // isReadOnlySafeRequest in server/app.go.
  if (options?.readOnlySafe) {
    return;
  }

  // Dynamic import is avoided here to prevent a module cycle during startup.
  // isMobileRuntime is imported from providers/runtime rather than the providers
  // barrel for the same reason: the barrel pulls in the providers, which import
  // this module.
  const readOnly = typeof window !== 'undefined' && window.__PLAINSHELF_READ_ONLY__ === true;
  if (readOnly) {
    throw new ApiError('Server is in read-only mode. Write operations are disabled.');
  }

  if (isMobileRuntime()) {
    throw new ApiError('The mobile app is read-only. Write operations are disabled.');
  }
}

export function buildApiUrl(path: string): string {
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `${apiBase}${normalized}`;
}

export function getActiveShelfID(): string {
  return activeShelfID;
}

export function setActiveShelfID(shelfID: string): void {
  const normalized = shelfID.trim();
  activeShelfID = normalized;
  if (typeof window !== 'undefined') {
    if (normalized) {
      window.localStorage.setItem(SHELF_STORAGE_KEY, normalized);
    } else {
      window.localStorage.removeItem(SHELF_STORAGE_KEY);
    }
  }
}

export function buildShelfApiPath(path: string, shelfID = getActiveShelfID()): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  const resolvedShelfID = shelfID.trim();
  if (!resolvedShelfID) {
    throw new ApiError('No shelf selected.');
  }
  return `/api/shelves/${encodeURIComponent(resolvedShelfID)}${normalizedPath}`;
}

async function getApiToken(): Promise<string> {
  // First non-empty source wins. `??` alone is not enough: the mobile provider
  // resolves getApiToken to '' when no token is stored, and an empty string is
  // not nullish, so it would mask a server-injected token instead of falling
  // through to it.
  const candidates = [
    await window.plainshelf?.getApiToken?.(),
    window.__PLAINSHELF_SECURITY__?.token,
    ENV_API_TOKEN
  ];

  for (const candidate of candidates) {
    const token = String(candidate ?? '').trim();
    if (token) {
      return token;
    }
  }

  return '';
}

async function withApiHeaders(init?: RequestInit): Promise<RequestInit> {
  const headers = new Headers(init?.headers ?? {});
  const token = await getApiToken();
  const tokenHeader = window.__PLAINSHELF_SECURITY__?.tokenHeader?.trim() || 'X-PlainShelf-Token';

  if (token && !headers.has(tokenHeader) && !headers.has('Authorization')) {
    headers.set(tokenHeader, token);
  }

  return {
    ...init,
    headers
  };
}

const FETCH_TIMEOUT_MS = 30_000;
// Large content (book text, cover images) on slow SMB mounts may take several minutes.
const FETCH_STREAM_TIMEOUT_MS = 300_000;

async function toApiError(res: Response): Promise<ApiError> {
  const raw = (await res.text()).trim();
  const message = raw || `HTTP ${res.status}: ${res.statusText}`;
  return new ApiError(message, {
    status: res.status,
    statusText: res.statusText,
    url: res.url
  });
}

async function fetchWithTimeout(url: string, init?: RequestInit, timeoutMs = FETCH_TIMEOUT_MS): Promise<Response> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...init, signal: controller.signal });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError('Request timed out — the shelf may be slow or unavailable.', {
        isTimeout: true,
        cause: err
      });
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

export interface FetchJsonOptions {
  // acceptStatuses lists non-2xx statuses whose JSON body is a normal result
  // rather than an error, such as a 409 that reports the task already running.
  acceptStatuses?: number[];
  // Uploads and other streaming requests can legitimately outlive the normal
  // metadata request timeout, especially when the shelf is on a sync mount.
  timeoutMs?: number;
  // readOnlySafe marks a POST that changes nothing on disk, so a read-only
  // server and the read-only mobile shell both accept it. Only the shelf rescan
  // endpoint qualifies today.
  readOnlySafe?: boolean;
}

export async function fetchJson<T>(
  path: string,
  init?: RequestInit,
  options?: FetchJsonOptions
): Promise<T> {
  assertApiMode();
  assertWritableRequest(init, options);

  const requestInit = await withApiHeaders(init);
  const headers = new Headers(requestInit.headers ?? {});
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }

  const res = await fetchWithTimeout(buildApiUrl(path), {
    ...requestInit,
    headers
  }, options?.timeoutMs ?? FETCH_TIMEOUT_MS);

  if (!res.ok && !options?.acceptStatuses?.includes(res.status)) {
    throw await toApiError(res);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const raw = await res.text();
  if (!raw.trim()) {
    return undefined as T;
  }

  try {
    return JSON.parse(raw) as T;
  } catch (cause) {
    throw new ApiError('Invalid JSON response from server.', {
      status: res.status,
      statusText: res.statusText,
      url: res.url,
      cause
    });
  }
}

export async function fetchText(path: string, init?: RequestInit): Promise<string> {
  assertApiMode();
  assertWritableRequest(init);

  const res = await fetchWithTimeout(buildApiUrl(path), await withApiHeaders(init), FETCH_STREAM_TIMEOUT_MS);
  if (!res.ok) {
    throw await toApiError(res);
  }

  return await res.text();
}

export async function fetchBlob(path: string, init?: RequestInit): Promise<Blob> {
  assertApiMode();
  assertWritableRequest(init);

  const res = await fetchWithTimeout(buildApiUrl(path), await withApiHeaders(init), FETCH_STREAM_TIMEOUT_MS);
  if (!res.ok) {
    throw await toApiError(res);
  }

  return await res.blob();
}
