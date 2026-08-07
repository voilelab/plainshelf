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

/** The account's top-level folder. Recursive listing is rejected on this one. */
const ROOT_FOLDER_ID = 0;

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

/** How much of an unexpected reply to quote back. Enough to recognise, short
 *  enough to read on a phone. */
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

/** Names of the folders at a level, for an error that has to be actionable. */
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

        // Every pCloud reply carries `result`. Treating its absence as success
        // would let a reply from somewhere else — a proxy, an interception
        // layer, an error page that happens to be JSON — flow on as an empty
        // listing, and surface much later as a confusing "folder not found".
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

  /**
   * Runs one request and hands the response to `handle` while the time budget
   * and the caller's cancellation are still in force.
   *
   * `fetch` resolves as soon as the response headers arrive, so a helper that
   * merely returned the Response would tear down its timer and abort listener
   * before the body — the slow part for a book's text or a cover — had been
   * read. Consuming inside this scope keeps a stalled transfer bounded and
   * keeps cancellation effective for the whole download.
   */
  private async withRequest<T>(
    url: string,
    init: RequestInit,
    timeoutMs: number,
    signal: AbortSignal | undefined,
    label: string,
    handle: (res: Response) => Promise<T>
  ): Promise<T> {
    const controller = new AbortController();
    // addEventListener never replays an abort that already happened, so a
    // signal cancelled before this call would otherwise be ignored entirely.
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
      // The caller cancelled: surface that as-is so it is never mistaken for a
      // transient failure and retried.
      if (signal?.aborted) {
        throw cause;
      }
      if (controller.signal.aborted) {
        throw new PCloudError(`pCloud ${label} timed out.`, { cause, retryable: true });
      }
      if (cause instanceof PCloudError) {
        throw cause;
      }
      // A dropped connection or an unreadable body. Worth another attempt.
      throw new PCloudError(`pCloud ${label} failed: ${(cause as Error)?.message ?? 'network error'}`, {
        cause,
        retryable: true
      });
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
    }
  }

  /** Lists one folder by id. */
  async listFolder(
    folderid: number,
    options?: { recursive?: boolean; signal?: AbortSignal }
  ): Promise<PCloudItem> {
    const params: PCloudParams = { folderid };
    if (options?.recursive) {
      params.recursive = 1;
    }

    const res = await this.call<PCloudListFolderResult>('listfolder', params, options?.signal);
    if (!res.metadata?.isfolder) {
      throw new PCloudError(
        `pCloud reported success for folder ${folderid} but returned no folder: ${excerpt(res)}`
      );
    }

    return res.metadata;
  }

  /**
   * Resolves a slash-separated folder path to its pCloud folder id.
   *
   * Walks one segment at a time from the account root instead of handing the
   * whole path to the API. `listfolder` is addressed by folder id — the
   * reference implementation (rclone's pCloud backend) never sends a path and
   * resolves every one this way — and a path-based listing answers "Directory
   * does not exist" even for a folder that plainly exists.
   *
   * Naming the segment that failed matters here: this runs while the user is
   * typing a folder into the connection form, and a bare "directory does not
   * exist" for a path they can see in pCloud is no help at all.
   */
  async resolveFolderID(path: string, signal?: AbortSignal): Promise<number> {
    const segments = path.split('/').filter((segment) => segment.length > 0);
    let folderid = ROOT_FOLDER_ID;
    let walked = '';

    for (const segment of segments) {
      const listing = await this.listFolder(folderid, { signal });
      const folders = (listing.contents ?? []).filter(
        (item) => item.isfolder && item.folderid !== undefined
      );
      const match = folders.find((item) => item.name === segment);

      if (!match || match.folderid === undefined) {
        // Naming what is there turns "we disagree about your account" into
        // something the user can act on — a typo, or the wrong pCloud account
        // signed in on this device.
        throw new PCloudError(
          `pCloud has no folder named "${segment}" in ${walked === '' ? 'your account root' : `"${walked}"`}. ` +
            describeFolders(folders)
        );
      }

      folderid = match.folderid;
      walked = `${walked}/${segment}`;
    }

    return folderid;
  }

  /**
   * Lists a folder and everything beneath it.
   *
   * One request covers the whole tree once the folder id is known. The account
   * root rejects recursive listing, so that case is walked manually — the
   * returned shape is identical either way, which keeps the parsing layer
   * unaware of which path was taken.
   */
  async listFolderRecursive(
    target: { path: string } | { folderid: number },
    signal?: AbortSignal
  ): Promise<PCloudItem> {
    const folderid = 'path' in target ? await this.resolveFolderID(target.path, signal) : target.folderid;

    if (folderid !== ROOT_FOLDER_ID) {
      try {
        return await this.listFolder(folderid, { recursive: true, signal });
      } catch (err) {
        // Kept as a safety net now that the root is handled up front: the API
        // may refuse a recursive listing somewhere this code does not predict.
        if (!(err instanceof PCloudError) || err.result !== PCLOUD_RESULT_RECURSIVE_ROOT_UNSUPPORTED) {
          throw err;
        }
      }
    }

    const root = await this.listFolder(folderid, { signal });
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
    return await this.download(fileid, signal, (res) => res.blob());
  }

  async downloadText(fileid: number, signal?: AbortSignal): Promise<string> {
    return await this.download(fileid, signal, (res) => res.text());
  }

  private async download<T>(
    fileid: number,
    signal: AbortSignal | undefined,
    consume: (res: Response) => Promise<T>
  ): Promise<T> {
    const url = await this.getFileLink(fileid, signal);

    // No Authorization header: the download host honours the link's own
    // credentials, so sending the API token would expose it to a different host
    // for no benefit.
    return await this.withRequest(url, {}, DOWNLOAD_TIMEOUT_MS, signal, 'download', async (res) => {
      if (!res.ok) {
        throw new PCloudError(`pCloud download failed: HTTP ${res.status} ${res.statusText}`, {
          status: res.status
        });
      }
      return await consume(res);
    });
  }
}
