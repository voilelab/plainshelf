import { ApiError } from '@/api/client';
import { reportErrorIncident } from './useErrorIncident';

/**
 * One auto-retry policy for reads that race the shelf's initial scan.
 *
 * A shelf still scanning answers 503 (`ErrShelfInitializing`) for every read,
 * and four independent loaders — the book listing, the folder tree, the
 * char-count index and the dashboard — each have to wait it out. They had a
 * copy of this timer and counter each, which is how their delays drifted apart.
 *
 * What to show once the budget is spent stays at the call site: the loaders
 * disagree on that on purpose (an "unreachable" flag, a generic error, a
 * specific string), and it is the only part of this that is theirs.
 *
 * `useShelvesStore` deliberately does not use this: its 20 × 300ms loop is a
 * fast startup poll for the shelf list, not a "scanning, wait a while" budget,
 * and giving it these delays would visibly slow the sidebar's first paint.
 */

export const SHELF_INIT_MAX_AUTO_RETRIES = 10; // ~30s of auto-retry before giving up
export const SHELF_INIT_RETRY_DELAY_MS = 3000;

/** True for the 503 a shelf answers while its initial scan is still running. */
export function isShelfInitializing(err: unknown): boolean {
  return err instanceof ApiError && err.status === 503;
}

interface ShelfInitRetry {
  /** True while a scheduled retry is waiting to fire. */
  readonly pending: boolean;
  /**
   * Schedules `attempt` after the shared delay and reports whether it did.
   * False means the budget is spent and the caller owns the failure state.
   *
   * `giveUpOn` is the refusal that prompted this call. api/client.ts publishes
   * no reference for a refusal carrying Retry-After, because a hidden retry
   * usually succeeds - but the attempt that spends the budget is not hidden,
   * and the reference has to come back with the error the caller then shows.
   */
  schedule(attempt: () => void, giveUpOn: unknown): boolean;
  /** Cancels a pending retry, keeping the remaining budget. */
  cancel(): void;
  /** Cancels a pending retry and restores the full budget. */
  reset(): void;
}

export function createShelfInitRetry(): ShelfInitRetry {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let attempts = 0;

  function cancel(): void {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
  }

  return {
    get pending(): boolean {
      return timer !== null;
    },
    cancel,
    reset(): void {
      cancel();
      attempts = 0;
    },
    schedule(attempt: () => void, giveUpOn: unknown): boolean {
      cancel();
      attempts++;
      if (attempts >= SHELF_INIT_MAX_AUTO_RETRIES) {
        reportErrorIncident(giveUpOn);
        return false;
      }
      timer = setTimeout(() => {
        timer = null;
        attempt();
      }, SHELF_INIT_RETRY_DELAY_MS);
      return true;
    }
  };
}
