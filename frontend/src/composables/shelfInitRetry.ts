import { ApiError } from '@/api/client';
import { reportErrorIncident } from './useErrorIncident';


export const SHELF_INIT_MAX_AUTO_RETRIES = 10;
export const SHELF_INIT_RETRY_DELAY_MS = 3000;

export function isShelfInitializing(err: unknown): boolean {
  return err instanceof ApiError && err.status === 503;
}

interface ShelfInitRetry {
  readonly pending: boolean;
  schedule(attempt: () => void, terminalRefusal: unknown): boolean;
  cancel(): void;
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
    schedule(attempt: () => void, terminalRefusal: unknown): boolean {
      cancel();
      attempts++;
      if (attempts >= SHELF_INIT_MAX_AUTO_RETRIES) {
        reportErrorIncident(terminalRefusal);
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
