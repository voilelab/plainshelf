import { reportIncident } from '@/composables/useErrorIncident';
import { isMobileRuntime } from '@/providers/runtime';

export class ApiError extends Error {
  status?: number;
  statusText?: string;
  url?: string;
  isTimeout: boolean;
  code?: string;
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

let apiBase = API_BASE_NORMALIZED;

export function getApiBase(): string {
  return apiBase;
}

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

export function isInsecurePublicAccess(): boolean {
  return typeof window !== 'undefined' && window.__PLAINSHELF_SECURITY__?.insecurePublicAccess === true;
}

function assertApiMode(): void {
  if (API_MODE === 'mock' && !IS_DEV) {
    throw new Error('Mock API mode is only allowed in development.');
  }
}


let apiFetch: typeof fetch = (...args) => fetch(...args);
let mobileWritesAllowed = false;

export function setApiTransport(transport: { fetch: typeof fetch; allowMobileWrites: boolean }): void {
  apiFetch = transport.fetch;
  mobileWritesAllowed = transport.allowMobileWrites;
}

function assertWritableRequest(init?: RequestInit, options?: FetchJsonOptions): void {
  const method = String(init?.method ?? 'GET').toUpperCase();
  if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) {
    return;
  }

  if (options?.readOnlySafe) {
    return;
  }

  const readOnly = typeof window !== 'undefined' && window.__PLAINSHELF_READ_ONLY__ === true;
  if (readOnly) {
    throw new ApiError('Server is in read-only mode. Write operations are disabled.');
  }

  if (isMobileRuntime() && !mobileWritesAllowed) {
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
const FETCH_STREAM_TIMEOUT_MS = 300_000;

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

const REQUEST_ID_HEADER = 'X-Request-Id';

function responseIncident(res: Response, fromEnvelope?: string): string | undefined {
  return fromEnvelope || res.headers.get(REQUEST_ID_HEADER)?.trim() || undefined;
}

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
    return await apiFetch(url, { ...init, signal: controller.signal });
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
  acceptStatuses?: number[];
  timeoutMs?: number;
  readOnlySafe?: boolean;
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
