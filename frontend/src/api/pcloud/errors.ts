/**
 * pCloud reports failures two ways: an HTTP-level error, or HTTP 200 carrying a
 * non-zero `result` code in the JSON body. PCloudError keeps both so callers can
 * tell a rate limit from a permanent restriction without re-reading the
 * response.
 *
 * This deliberately does not extend ApiError from '@/api/client'. That module
 * reads import.meta.env and window.localStorage at import time and models the
 * PlainShelf server's base URL and token scheme, none of which applies to a
 * direct pCloud session. The provider layer translates at its own boundary.
 */
export class PCloudError extends Error {
  /** pCloud result code, or 0 when the failure was at the HTTP/transport layer. */
  readonly result: number;
  readonly status?: number;

  constructor(message: string, options?: { result?: number; status?: number; cause?: unknown }) {
    super(message);
    this.name = 'PCloudError';
    this.result = options?.result ?? 0;
    this.status = options?.status;

    if (options?.cause !== undefined) {
      (this as Error & { cause?: unknown }).cause = options.cause;
    }
  }
}

/**
 * Listing the account root (folderid 0) recursively is rejected by the API.
 * A shelf normally lives in a sub-folder so this is rare, but it has to be
 * handled rather than surfaced, because the caller cannot avoid it by retrying.
 */
export const PCLOUD_RESULT_RECURSIVE_ROOT_UNSUPPORTED = 1101;

const RETRYABLE_HTTP_STATUS = new Set([429, 500, 502, 503, 504, 509]);

/**
 * pCloud groups result codes by their leading digit: 4xxx is rate limiting and
 * 5xxx is an internal error, both of which are worth another attempt. 1101 is a
 * permanent restriction and must not be retried even though it is a 1xxx code.
 *
 * Classification verified against rclone's pCloud backend
 * (rclone/rclone, backend/pcloud/pcloud.go, shouldRetry).
 */
export function isRetryablePCloudError(err: unknown): boolean {
  if (!(err instanceof PCloudError)) {
    return false;
  }

  if (err.result === PCLOUD_RESULT_RECURSIVE_ROOT_UNSUPPORTED) {
    return false;
  }

  const group = Math.floor(err.result / 1000);
  if (group === 4 || group === 5) {
    return true;
  }

  return err.status !== undefined && RETRYABLE_HTTP_STATUS.has(err.status);
}
