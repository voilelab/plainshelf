import { PCloudError, isRetryablePCloudError } from './errors';
import type { PCloudApiHost, PCloudItem, PCloudListFolderResult, PCloudUserInfoResult } from './types';

const REQUEST_TIMEOUT_MS = 30_000;

const MAX_ATTEMPTS = 3;
const RETRY_BASE_DELAY_MS = 500;

export const ROOT_FOLDER_ID = 0;

type PCloudParams = Record<string, string | number | boolean | undefined>;

interface PCloudClientOptions {
  host: PCloudApiHost | string;
  accessToken: string;
  fetchImpl?: typeof fetch;
  sleepImpl?: (ms: number) => Promise<void>;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

const MAX_EXCERPT_LENGTH = 200;

function excerpt(value: unknown): string {
  let text: string;
  try {
    text = JSON.stringify(value) ?? String(value);
  } catch {
    text = String(value);
  }
  return text.length > MAX_EXCERPT_LENGTH ? `${text.slice(0, MAX_EXCERPT_LENGTH)}…` : text;
}

const MAX_LISTED_FOLDERS = 10;

function describeFolders(folders: PCloudItem[]): string {
  if (folders.length === 0) {
    return 'That level contains no folders at all.';
  }

  const names = [...folders].map((folder) => folder.name).sort((a, b) => a.localeCompare(b));
  const shown = names.slice(0, MAX_LISTED_FOLDERS).map((name) => `"${name}"`).join(', ');
  const remaining = names.length - MAX_LISTED_FOLDERS;

  return remaining > 0 ? `It contains ${shown} and ${remaining} more.` : `It contains ${shown}.`;
}

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
    return await this.withRequest(
      this.buildUrl(method, params),
      { headers: { Authorization: `Bearer ${this.accessToken}` } },
      REQUEST_TIMEOUT_MS,
      signal,
      `${method} request`,
      async (res) => {
        if (!res.ok) {
          throw new PCloudError(`pCloud ${method} failed: HTTP ${res.status} ${res.statusText}`, {
            status: res.status
          });
        }

        const payload = (await res.json()) as { result?: unknown; error?: unknown };

        if (typeof payload?.result !== 'number') {
          throw new PCloudError(
            `pCloud ${method} returned a reply that did not come from the pCloud API: ${excerpt(payload)}`
          );
        }

        if (payload.result !== 0) {
          throw new PCloudError(
            `pCloud ${method} failed: ${payload.error ?? 'unknown error'} (${payload.result})`,
            { result: payload.result }
          );
        }

        return payload as T;
      }
    );
  }

  private async withRequest<T>(
    url: string,
    init: RequestInit,
    timeoutMs: number,
    signal: AbortSignal | undefined,
    label: string,
    handle: (res: Response) => Promise<T>
  ): Promise<T> {
    const controller = new AbortController();
    // DOM Standard AbortSignal does not replay an abort event to listeners registered after abortion.
    if (signal?.aborted) {
      controller.abort();
    }
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    const onAbort = () => controller.abort();
    signal?.addEventListener('abort', onAbort);

    try {
      const res = await this.fetchImpl(url, { ...init, signal: controller.signal });
      return await handle(res);
    } catch (cause) {
      if (signal?.aborted) {
        throw cause;
      }
      if (controller.signal.aborted) {
        throw new PCloudError(`pCloud ${label} timed out.`, { cause, retryable: true });
      }
      if (cause instanceof PCloudError) {
        throw cause;
      }
      throw new PCloudError(`pCloud ${label} failed: ${(cause as Error)?.message ?? 'network error'}`, {
        cause,
        retryable: true
      });
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
    }
  }

  async listFolder(folderid: number, options?: { signal?: AbortSignal }): Promise<PCloudItem> {
    const res = await this.call<PCloudListFolderResult>('listfolder', { folderid }, options?.signal);
    if (!res.metadata?.isfolder) {
      throw new PCloudError(
        `pCloud reported success for folder ${folderid} but returned no folder: ${excerpt(res)}`
      );
    }

    return res.metadata;
  }

  async getUserInfo(signal?: AbortSignal): Promise<PCloudUserInfoResult> {
    const info = await this.call<PCloudUserInfoResult>('userinfo', {}, signal);
    if (!info.email && info.userid === undefined) {
      throw new PCloudError(`pCloud userinfo returned no account identity: ${excerpt(info)}`);
    }
    return info;
  }

  async resolveFolderTrail(
    path: string,
    signal?: AbortSignal
  ): Promise<Array<{ name: string; folderid: number }>> {
    const segments = path.split('/').filter((segment) => segment.length > 0);
    const trail: Array<{ name: string; folderid: number }> = [];
    let folderid = ROOT_FOLDER_ID;
    let walked = '';

    for (const segment of segments) {
      const listing = await this.listFolder(folderid, { signal });
      const folders = (listing.contents ?? []).filter(
        (item) => item.isfolder && item.folderid !== undefined
      );
      const match = folders.find((item) => item.name === segment);

      if (!match || match.folderid === undefined) {
        throw new PCloudError(
          `pCloud has no folder named "${segment}" in ${walked === '' ? 'your account root' : `"${walked}"`}. ` +
            describeFolders(folders)
        );
      }

      folderid = match.folderid;
      walked = `${walked}/${segment}`;
      trail.push({ name: segment, folderid });
    }

    return trail;
  }
}
