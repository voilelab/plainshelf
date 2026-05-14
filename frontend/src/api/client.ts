export class ApiError extends Error {
  status?: number;
  statusText?: string;
  url?: string;

  constructor(
    message: string,
    options?: {
      status?: number;
      statusText?: string;
      url?: string;
      cause?: unknown;
    }
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = options?.status;
    this.statusText = options?.statusText;
    this.url = options?.url;

    if (options?.cause !== undefined) {
      (this as Error & { cause?: unknown }).cause = options.cause;
    }
  }
}

type ApiMode = 'live' | 'mock';

const RAW_API_BASE = String(import.meta.env.VITE_API_BASE ?? '').trim();
const API_BASE_NORMALIZED = RAW_API_BASE.replace(/\/+$/, '');
const USE_MOCK_OPT_IN = String(import.meta.env.VITE_USE_MOCK_API ?? '').toLowerCase() === 'true';
const IS_DEV = import.meta.env.DEV;

if (USE_MOCK_OPT_IN && !IS_DEV) {
  throw new Error('VITE_USE_MOCK_API=true is development-only and cannot be enabled in production.');
}

const API_MODE: ApiMode = IS_DEV && USE_MOCK_OPT_IN ? 'mock' : 'live';

if (IS_DEV && API_MODE === 'mock') {
  console.info('[api] MOCK API mode enabled (VITE_USE_MOCK_API=true).');
}

export const API_BASE = API_BASE_NORMALIZED;

export function isMockApiMode(): boolean {
  return API_MODE === 'mock';
}

export function getApiModeLabel(): string {
  return API_MODE;
}

export function assertApiMode(): void {
  if (API_MODE === 'mock' && !IS_DEV) {
    throw new Error('Mock API mode is only allowed in development.');
  }
}

export function buildApiUrl(path: string): string {
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `${API_BASE}${normalized}`;
}

async function toApiError(res: Response): Promise<ApiError> {
  const raw = (await res.text()).trim();
  const message = raw || `HTTP ${res.status}: ${res.statusText}`;
  return new ApiError(message, {
    status: res.status,
    statusText: res.statusText,
    url: res.url
  });
}

export async function fetchJson<T>(path: string, init?: RequestInit): Promise<T> {
  assertApiMode();

  const headers = new Headers(init?.headers ?? {});
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }

  const res = await fetch(buildApiUrl(path), {
    ...init,
    headers
  });

  if (!res.ok) {
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

  const res = await fetch(buildApiUrl(path), init);
  if (!res.ok) {
    throw await toApiError(res);
  }

  return await res.text();
}

export async function fetchBlob(path: string, init?: RequestInit): Promise<Blob> {
  assertApiMode();

  const res = await fetch(buildApiUrl(path), init);
  if (!res.ok) {
    throw await toApiError(res);
  }

  return await res.blob();
}
