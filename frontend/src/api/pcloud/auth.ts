import { PCloudError } from './errors';
import { PCLOUD_API_HOSTS, PCLOUD_DEFAULT_HOST } from './types';
import type { PCloudApiHost, PCloudTokenResult } from './types';

// The pCloud SDK OAuth poll flow uses poll_token without a redirect URI or client secret and polls both regional endpoints (src/oauth/index.js).
const AUTHORIZE_URL = 'https://my.pcloud.com/oauth2/authorize';

const REQUEST_ID_LENGTH = 40;
const REQUEST_ID_ALPHABET = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';

const DEFAULT_AUTH_TIMEOUT_MS = 5 * 60_000;
const POLL_INTERVAL_MS = 2_000;
const POLL_REQUEST_TIMEOUT_MS = 60_000;

interface PCloudSession {
  accessToken: string;
  host: PCloudApiHost;
}

interface PCloudAuthOptions {
  clientId: string;
  requestId: string;
  signal?: AbortSignal;
  timeoutMs?: number;
  fetchImpl?: typeof fetch;
  sleepImpl?: (ms: number) => Promise<void>;
  nowImpl?: () => number;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function generateRequestId(): string {
  const bytes = new Uint8Array(REQUEST_ID_LENGTH);
  const cryptoObj = globalThis.crypto;

  if (!cryptoObj?.getRandomValues) {
    throw new PCloudError('A secure random source is required to start pCloud authorization.');
  }
  cryptoObj.getRandomValues(bytes);

  let out = '';
  for (const byte of bytes) {
    out += REQUEST_ID_ALPHABET[byte % REQUEST_ID_ALPHABET.length];
  }
  return out;
}

export function buildAuthorizeUrl(clientId: string, requestId: string): string {
  const trimmedClientId = clientId.trim();
  if (!trimmedClientId) {
    throw new PCloudError('A pCloud client_id is required to start authorization.');
  }
  if (!requestId.trim()) {
    throw new PCloudError('A request_id is required to start authorization.');
  }

  const url = new URL(AUTHORIZE_URL);
  url.searchParams.set('client_id', trimmedClientId);
  url.searchParams.set('request_id', requestId);
  url.searchParams.set('response_type', 'poll_token');
  return url.toString();
}

export function hostForLocationId(locationId: number | undefined): PCloudApiHost {
  return locationId === 2 ? 'eapi.pcloud.com' : PCLOUD_DEFAULT_HOST;
}

export async function pollForToken(options: PCloudAuthOptions): Promise<PCloudSession> {
  const {
    clientId,
    requestId,
    signal,
    timeoutMs = DEFAULT_AUTH_TIMEOUT_MS,
    fetchImpl = ((...args: Parameters<typeof fetch>) => fetch(...args)) as typeof fetch,
    sleepImpl = defaultSleep,
    nowImpl = () => Date.now()
  } = options;

  const deadline = nowImpl() + timeoutMs;
  const raceController = new AbortController();
  // DOM Standard AbortSignal does not replay an abort event to listeners registered after abortion.
  if (signal?.aborted) {
    raceController.abort();
  }
  const onAbort = () => raceController.abort();
  signal?.addEventListener('abort', onAbort);

  const attempts = PCLOUD_API_HOSTS.map((host) =>
    pollHost({
      host,
      clientId,
      requestId,
      deadline,
      signal: raceController.signal,
      fetchImpl,
      sleepImpl,
      nowImpl
    })
  );

  try {
    return await firstFulfilled(attempts);
  } finally {
    raceController.abort();
    signal?.removeEventListener('abort', onAbort);
  }
}

interface PollHostOptions {
  host: PCloudApiHost;
  clientId: string;
  requestId: string;
  deadline: number;
  signal: AbortSignal;
  fetchImpl: typeof fetch;
  sleepImpl: (ms: number) => Promise<void>;
  nowImpl: () => number;
}

async function pollHost(options: PollHostOptions): Promise<PCloudSession> {
  const { host, clientId, requestId, deadline, signal, fetchImpl, sleepImpl, nowImpl } = options;

  const url = new URL(`https://${host}/oauth2_token`);
  url.searchParams.set('client_id', clientId);
  url.searchParams.set('request_id', requestId);

  while (!signal.aborted && nowImpl() < deadline) {
    const token = await pollOnce(url.toString(), signal, fetchImpl);
    if (token) {
      // The pCloud SDK forwards response.locationid from either regional polling endpoint (src/oauth/index.js).
      return {
        accessToken: token.accessToken,
        host: token.locationId === undefined ? host : hostForLocationId(token.locationId)
      };
    }

    if (signal.aborted) {
      break;
    }
    await sleepImpl(POLL_INTERVAL_MS);
  }

  throw new PCloudError(
    signal.aborted
      ? 'pCloud authorization was cancelled.'
      : `pCloud authorization timed out waiting for approval (${host}).`
  );
}

async function pollOnce(
  url: string,
  signal: AbortSignal,
  fetchImpl: typeof fetch
): Promise<{ accessToken: string; locationId?: number } | null> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), POLL_REQUEST_TIMEOUT_MS);
  const onAbort = () => controller.abort();
  signal.addEventListener('abort', onAbort);

  try {
    const res = await fetchImpl(url, { signal: controller.signal });
    if (!res.ok) {
      return null;
    }

    const payload = (await res.json()) as PCloudTokenResult;
    if (payload.result !== 0 || !payload.access_token) {
      return null;
    }

    return { accessToken: payload.access_token, locationId: payload.locationid };
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', onAbort);
  }
}

function firstFulfilled<T>(promises: Promise<T>[]): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    let pending = promises.length;
    let lastError: unknown;

    if (pending === 0) {
      reject(new PCloudError('No pCloud host was polled.'));
      return;
    }

    for (const promise of promises) {
      promise.then(resolve, (err) => {
        lastError = err;
        pending -= 1;
        if (pending === 0) {
          reject(lastError);
        }
      });
    }
  });
}
