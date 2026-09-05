import { reportIncident } from '@/composables/useErrorIncident';
import { isMobileRuntime } from '@/providers/runtime';

export class ApiError extends Error {
  status?: number;
  statusText?: string;
  url?: string;
  isTimeout: boolean;
  /**
   * The server's stable error code (SCREAMING_SNAKE), when the response carried
   * the JSON error envelope. Unset for the routes that still answer plain text
   * and for transport failures, so callers must treat it as optional.
   */
  code?: string;
  /**
   * The reference a bug report quotes. For a server failure it is the request's
   * ID, so it names the log line that carries the cause the body withheld; for
   * a failure the frontend raised itself it is a `c-` ID (see api/incident.ts).
   * Unset where neither is available, so callers must treat it as optional.
   */
  incident?: string;

  constructor(
    message: string,
    options?: {
      status?: number;
      statusText?: string;
      url?: string;
      cause?: unknown;
      isTimeout?: boolean;
      code?: string;
      incident?: string;
    }
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = options?.status;
    this.statusText = options?.statusText;
    this.url = options?.url;
    this.isTimeout = options?.isTimeout ?? false;
    this.code = options?.code;
    this.incident = options?.incident;

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
      // Set by the Go server only when security mode is none and the listen
      // address is not loopback: the API answers every request, including
      // writes and deletes, without authentication. Drives the persistent
      // Web UI warning; see SecurityWarningBanner.vue.
      insecurePublicAccess?: boolean;
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
let apiBase = API_BASE_NORMALIZED;

export function getApiBase(): string {
  return apiBase;
}

/**
 * Exported because the stored and applied values have to agree exactly: on
 * mobile the applied base is half the key that scopes device-local book data
 * (providers/cacheScope.ts), so a trailing slash left on one side and stripped
 * on the other points at a different cache.
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

// The Go server sets this flag only for security mode none bound to a
// non-loopback address; every other posture leaves it unset.
export function isInsecurePublicAccess(): boolean {
  return typeof window !== 'undefined' && window.__PLAINSHELF_SECURITY__?.insecurePublicAccess === true;
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

  // Imported from providers/runtime rather than the providers barrel, and
  // statically: the barrel pulls in the providers, which import this module.
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

/**
 * The JSON body the Go server's error table answers with. Not every refusal
 * carries it — the routes that still call http.Error answer plain text, as does
 * the standalone reader — so it is parsed opportunistically with the raw text as
 * the fallback.
 */
interface ApiErrorEnvelope {
  error: { code?: unknown; message?: unknown; incident?: unknown };
}

function parseErrorEnvelope(raw: string): {
  code?: string;
  message?: string;
  incident?: string;
} | null {
  if (!raw.startsWith('{')) return null;

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }

  const envelope = (parsed as ApiErrorEnvelope | null)?.error;
  if (typeof envelope !== 'object' || envelope === null) return null;

  const code = typeof envelope.code === 'string' ? envelope.code : undefined;
  const message = typeof envelope.message === 'string' ? envelope.message : undefined;
  if (code === undefined && message === undefined) return null;

  const incident = typeof envelope.incident === 'string' ? envelope.incident : undefined;

  return { code, message, incident };
}

// Every response leaves the server through the request-ID middleware, so the
// plain-text refusals carry a reference even without an envelope to put it in.
const REQUEST_ID_HEADER = 'X-Request-Id';

function responseIncident(res: Response, fromEnvelope?: string): string | undefined {
  return fromEnvelope || res.headers.get(REQUEST_ID_HEADER)?.trim() || undefined;
}

// A refusal the server itself calls transient - the shelf is still scanning, a
// lock is held - is waited out and usually succeeds, so it must not leave a
// reference on screen for a failure the user never saw. The error still carries
// its incident, and shelfInitRetry publishes it on the attempt that spends the
// retry budget, which is the one the caller goes on to show.
function reportResponseIncident(res: Response, incident?: string): void {
  if (incident && !res.headers.has('Retry-After')) {
    reportIncident(incident);
  }
}

function apiErrorFrom(res: Response, raw: string): ApiError {
  const envelope = parseErrorEnvelope(raw);
  const message =
    envelope?.message || (envelope ? '' : raw) || `HTTP ${res.status}: ${res.statusText}`;
  const incident = responseIncident(res, envelope?.incident);
  reportResponseIncident(res, incident);

  return new ApiError(message, {
    status: res.status,
    statusText: res.statusText,
    url: res.url,
    code: envelope?.code,
    incident
  });
}

async function toApiError(res: Response): Promise<ApiError> {
  return apiErrorFrom(res, (await res.text()).trim());
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

interface FetchJsonOptions {
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
  // onResponse hands the caller the response before its body is read, for the
  // few routes whose answer is not only in the body. It is called for an error
  // response too, so a header is not missed on the path that throws.
  onResponse?: (res: Response) => void;
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

  options?.onResponse?.(res);

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

  // acceptStatuses says a body at that status *may* be a normal result, not
  // that every body at it is one: a 409 carries either the running chain's ID
  // or a refusal. The error envelope is self-identifying, so it is rejected
  // here whatever the status - otherwise the caller reads taskchain_id off a
  // refusal, gets undefined, and polls it.
  if (!res.ok && parseErrorEnvelope(raw.trim())) {
    throw apiErrorFrom(res, raw.trim());
  }

  try {
    return JSON.parse(raw) as T;
  } catch (cause) {
    const incident = responseIncident(res);
    reportResponseIncident(res, incident);

    throw new ApiError('Invalid JSON response from server.', {
      status: res.status,
      statusText: res.statusText,
      url: res.url,
      cause,
      incident
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
