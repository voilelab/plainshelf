import { PCloudError } from './errors';
import { PCLOUD_API_HOSTS, PCLOUD_DEFAULT_HOST } from './types';
import type { PCloudApiHost, PCloudTokenResult } from './types';

// The `poll_token` flow, as implemented by pCloud's own JS SDK
// (pCloud/pcloud-sdk-js, src/oauth/index.js).
//
// It is the only one of pCloud's three OAuth flows that suits a packaged mobile
// app: it needs no redirect_uri and no app secret, so there is no custom URL
// scheme to register and nothing confidential to ship inside the APK. The app
// opens an authorization URL in the system browser and simultaneously long-polls
// `oauth2_token` on both regional hosts; whichever one answers both delivers the
// token and identifies the account's region.
const AUTHORIZE_URL = 'https://my.pcloud.com/oauth2/authorize';

const REQUEST_ID_LENGTH = 40;
const REQUEST_ID_ALPHABET = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';

/** How long to keep polling before giving up on the user completing the flow. */
export const DEFAULT_AUTH_TIMEOUT_MS = 5 * 60_000;
const POLL_INTERVAL_MS = 2_000;
const POLL_REQUEST_TIMEOUT_MS = 60_000;

/** An authorized pCloud session: a token plus the region that issued it. */
export interface PCloudSession {
  accessToken: string;
  host: PCloudApiHost;
}

export interface PCloudAuthOptions {
  clientId: string;
  requestId: string;
  signal?: AbortSignal;
  timeoutMs?: number;
  fetchImpl?: typeof fetch;
  sleepImpl?: (ms: number) => Promise<void>;
  /** Overridable so tests do not depend on wall-clock time. */
  nowImpl?: () => number;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Generates the `request_id` that ties the browser authorization to this
 * device's poll. It is the only thing preventing another client from claiming
 * the token, so it must come from a CSPRNG.
 */
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

/**
 * Builds the URL the user must open to approve access. The caller is
 * responsible for opening it in the system browser — keeping that out of here
 * leaves this module free of Capacitor imports and testable under Node.
 */
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

/** Maps pCloud's numeric region id onto the API host that serves it. */
export function hostForLocationId(locationId: number | undefined): PCloudApiHost {
  return locationId === 2 ? 'eapi.pcloud.com' : PCLOUD_DEFAULT_HOST;
}

/**
 * Waits for the user to approve the request and returns the resulting session.
 *
 * Both regional hosts are polled at once because the account's region is not
 * known until it answers. The host that returns the token is authoritative: it
 * is the one holding the account, so it — not a guess — becomes the session's
 * API host.
 */
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
  // Cancels the losing region's poll as soon as a winner is known, so a
  // successful sign-in does not leave a request hanging for the full timeout.
  const raceController = new AbortController();
  // addEventListener never replays an abort that already fired, so a signal
  // cancelled before this call would leave both polls running to the deadline.
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
      // The polling endpoint is global enough that either regional host can
      // deliver the token. pCloud's locationid identifies where the account's
      // files actually live, so it is authoritative whenever present. Keep the
      // answering host only as a compatibility fallback for an old response
      // that omits locationid.
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

/**
 * One poll round. A failure here is expected rather than exceptional — the
 * wrong-region host errors for the whole flow, and the right one errors until
 * the user approves — so transport and API errors alike mean "not yet".
 */
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

/**
 * Resolves with the first fulfilled promise and rejects only once every one has
 * rejected. Hand-rolled because the project targets ES2020, which predates
 * Promise.any.
 */
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
