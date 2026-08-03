import {
  PCLOUD_RESULT_RECURSIVE_ROOT_UNSUPPORTED,
  PCloudError,
  isRetryablePCloudError
} from './errors';
import type {
  PCloudApiHost,
  PCloudGetFileLinkResult,
  PCloudItem,
  PCloudListFolderResult
} from './types';

// Mirrors the two budgets in api/client.ts: metadata calls should fail fast,
// while a book's text or cover may take much longer on a slow connection.
const REQUEST_TIMEOUT_MS = 30_000;
const DOWNLOAD_TIMEOUT_MS = 300_000;

const MAX_ATTEMPTS = 3;
const RETRY_BASE_DELAY_MS = 500;

export type PCloudParams = Record<string, string | number | boolean | undefined>;

export interface PCloudClientOptions {
  host: PCloudApiHost | string;
  accessToken: string;
  /** Injectable for tests; defaults to the global fetch. */
  fetchImpl?: typeof fetch;
  /** Injectable for tests so retry backoff does not slow the suite down. */
  sleepImpl?: (ms: number) => Promise<void>;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * A thin, read-only pCloud API client.
 *
 * Auth travels as a Bearer header rather than an `access_token` query
 * parameter: both are accepted by pCloud, but only the header keeps the token
 * out of URLs, which end up in logs and in error messages.
 */
export class PCloudClient {
  private readonly host: string;
  private readonly accessToken: string;
  private readonly fetchImpl: typeof fetch;
  private readonly sleepImpl: (ms: number) => Promise<void>;

  constructor(options: PCloudClientOptions) {
    this.host = String(options.host).trim().replace(/^https?:\/\//, '').replace(/\/+$/, '');
    this.accessToken = options.accessToken.trim();
    this.fetchImpl = options.fetchImpl ?? ((...args) => fetch(...args));
    this.sleepImpl = options.sleepImpl ?? defaultSleep;

    if (!this.host) {
      throw new PCloudError('pCloud API host is required.');
    }
    if (!this.accessToken) {
      throw new PCloudError('pCloud access token is required.');
    }
  }

  private buildUrl(method: string, params: PCloudParams): string {
    const url = new URL(`https://${this.host}/${method}`);
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) {
        url.searchParams.set(key, String(value));
      }
    }
    return url.toString();
  }

  /**
   * Issues one API call and unwraps the `result` envelope.
   *
   * Retries only failures classified as transient, so a bad token or a
   * permanent restriction surfaces on the first attempt instead of after three
   * rounds of backoff.
   */
  async call<T>(method: string, params: PCloudParams = {}, signal?: AbortSignal): Promise<T> {
    let lastError: unknown;

    for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt += 1) {
      try {
        return await this.callOnce<T>(method, params, signal);
      } catch (err) {
        lastError = err;
        if (attempt === MAX_ATTEMPTS || !isRetryablePCloudError(err)) {
          throw err;
        }
        await this.sleepImpl(RETRY_BASE_DELAY_MS * 2 ** (attempt - 1));
      }
    }

    throw lastError;
  }

  private async callOnce<T>(method: string, params: PCloudParams, signal?: AbortSignal): Promise<T> {
    const res = await this.request(this.buildUrl(method, params), REQUEST_TIMEOUT_MS, signal);

    if (!res.ok) {
      throw new PCloudError(`pCloud ${method} failed: HTTP ${res.status} ${res.statusText}`, {
        status: res.status
      });
    }

    let payload: unknown;
    try {
      payload = await res.json();
    } catch (cause) {
      throw new PCloudError(`pCloud ${method} returned an invalid JSON response.`, { cause });
    }

    const envelope = payload as { result?: number; error?: string };
    const result = envelope?.result ?? 0;
    if (result !== 0) {
      throw new PCloudError(`pCloud ${method} failed: ${envelope?.error ?? 'unknown error'} (${result})`, {
        result
      });
    }

    return payload as T;
  }

  private async request(url: string, timeoutMs: number, signal?: AbortSignal): Promise<Response> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    const onAbort = () => controller.abort();
    signal?.addEventListener('abort', onAbort);

    try {
      return await this.fetchImpl(url, {
        headers: { Authorization: `Bearer ${this.accessToken}` },
        signal: controller.signal
      });
    } catch (cause) {
      if (signal?.aborted) {
        throw cause;
      }
      if (cause instanceof DOMException && cause.name === 'AbortError') {
        throw new PCloudError('pCloud request timed out.', { cause });
      }
      throw cause;
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
    }
  }

  /** Lists one folder. Pass either a shelf path or a folder id. */
  async listFolder(
    target: { path: string } | { folderid: number },
    options?: { recursive?: boolean; signal?: AbortSignal }
  ): Promise<PCloudItem> {
    const params: PCloudParams = 'path' in target ? { path: target.path } : { folderid: target.folderid };
    if (options?.recursive) {
      params.recursive = 1;
    }

    const res = await this.call<PCloudListFolderResult>('listfolder', params, options?.signal);
    return res.metadata;
  }

  /**
   * Lists a folder and everything beneath it.
   *
   * One request covers the whole tree in the normal case. The account root
   * rejects recursive listing, so that case is walked manually — the returned
   * shape is identical either way, which keeps the parsing layer unaware of
   * which path was taken.
   */
  async listFolderRecursive(
    target: { path: string } | { folderid: number },
    signal?: AbortSignal
  ): Promise<PCloudItem> {
    try {
      return await this.listFolder(target, { recursive: true, signal });
    } catch (err) {
      if (!(err instanceof PCloudError) || err.result !== PCLOUD_RESULT_RECURSIVE_ROOT_UNSUPPORTED) {
        throw err;
      }
    }

    const root = await this.listFolder(target, { signal });
    return { ...root, contents: await this.expandChildren(root.contents ?? [], signal) };
  }

  private async expandChildren(items: PCloudItem[], signal?: AbortSignal): Promise<PCloudItem[]> {
    const expanded: PCloudItem[] = [];

    for (const item of items) {
      if (!item.isfolder || item.folderid === undefined) {
        expanded.push(item);
        continue;
      }
      const child = await this.listFolderRecursive({ folderid: item.folderid }, signal);
      expanded.push({ ...item, contents: child.contents ?? [] });
    }

    return expanded;
  }

  /**
   * Resolves a file's download URL. The link expires, so it is fetched per
   * download and never cached alongside file metadata.
   */
  async getFileLink(fileid: number, signal?: AbortSignal): Promise<string> {
    const res = await this.call<PCloudGetFileLinkResult>('getfilelink', { fileid }, signal);
    const host = res.hosts?.[0];

    if (!host || !res.path) {
      throw new PCloudError('pCloud getfilelink returned no usable download host.');
    }

    return `https://${host}${res.path}`;
  }

  async downloadBlob(fileid: number, signal?: AbortSignal): Promise<Blob> {
    const res = await this.downloadResponse(fileid, signal);
    return await res.blob();
  }

  async downloadText(fileid: number, signal?: AbortSignal): Promise<string> {
    const res = await this.downloadResponse(fileid, signal);
    return await res.text();
  }

  private async downloadResponse(fileid: number, signal?: AbortSignal): Promise<Response> {
    const url = await this.getFileLink(fileid, signal);
    // The download host serves the link's own credentials; sending the API
    // Bearer token here would leak it to a different host for no benefit.
    const res = await this.requestWithoutAuth(url, DOWNLOAD_TIMEOUT_MS, signal);

    if (!res.ok) {
      throw new PCloudError(`pCloud download failed: HTTP ${res.status} ${res.statusText}`, {
        status: res.status
      });
    }

    return res;
  }

  private async requestWithoutAuth(url: string, timeoutMs: number, signal?: AbortSignal): Promise<Response> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    const onAbort = () => controller.abort();
    signal?.addEventListener('abort', onAbort);

    try {
      return await this.fetchImpl(url, { signal: controller.signal });
    } catch (cause) {
      if (signal?.aborted) {
        throw cause;
      }
      if (cause instanceof DOMException && cause.name === 'AbortError') {
        throw new PCloudError('pCloud download timed out.', { cause });
      }
      throw cause;
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
    }
  }
}
