import { newClientIncidentID } from '@/api/incident';

export class PCloudError extends Error {
  readonly result: number;
  readonly incident: string;
  readonly status?: number;
  readonly retryable: boolean;

  constructor(
    message: string,
    options?: { result?: number; status?: number; cause?: unknown; retryable?: boolean }
  ) {
    super(message);
    this.name = 'PCloudError';
    this.incident = newClientIncidentID();
    this.result = options?.result ?? 0;
    this.status = options?.status;
    this.retryable = options?.retryable ?? false;

    if (options?.cause !== undefined) {
      (this as Error & { cause?: unknown }).cause = options.cause;
    }
  }
}

const PCLOUD_RESULT_RECURSIVE_LISTING_UNSUPPORTED = 1101;

const RETRYABLE_HTTP_STATUS = new Set([429, 500, 502, 503, 504, 509]);

// rclone backend/pcloud/pcloud.go shouldRetry treats result groups 4 and 5 as retryable, except permanent result 1101.
export function isRetryablePCloudError(err: unknown): boolean {
  if (!(err instanceof PCloudError)) {
    return false;
  }

  if (err.result === PCLOUD_RESULT_RECURSIVE_LISTING_UNSUPPORTED) {
    return false;
  }

  if (err.retryable) {
    return true;
  }

  const group = Math.floor(err.result / 1000);
  if (group === 4 || group === 5) {
    return true;
  }

  return err.status !== undefined && RETRYABLE_HTTP_STATUS.has(err.status);
}
