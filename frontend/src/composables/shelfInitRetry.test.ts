import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@/api/client';
import { useErrorIncident } from './useErrorIncident';
import {
  SHELF_INIT_MAX_AUTO_RETRIES,
  SHELF_INIT_RETRY_DELAY_MS,
  createShelfInitRetry,
  isShelfInitializing
} from './shelfInitRetry';

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('isShelfInitializing', () => {
  it('matches only the 503 a scanning shelf answers with', () => {
    expect(isShelfInitializing(new ApiError('initializing', { status: 503 }))).toBe(true);
    expect(isShelfInitializing(new ApiError('gone', { status: 404 }))).toBe(false);
    expect(isShelfInitializing(new Error('boom'))).toBe(false);
    expect(isShelfInitializing(undefined)).toBe(false);
  });
});

// Every case schedules against the refusal a scanning shelf answers with; only
// the reference cases below care what is in it.
const scanning = () => new ApiError('initializing', { status: 503, incident: 'K7MQ4XZB' });

describe('createShelfInitRetry', () => {
  it('runs the attempt after the shared delay and not before', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    expect(retry.schedule(attempt, scanning())).toBe(true);
    expect(retry.pending).toBe(true);

    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS - 1);
    expect(attempt).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);
    expect(attempt).toHaveBeenCalledTimes(1);
    expect(retry.pending).toBe(false);
  });

  it('spends the budget across the whole sequence of attempts', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    // The counter is the number of failures seen, so the last one in the budget
    // is the one that gives up rather than scheduling again.
    for (let i = 1; i < SHELF_INIT_MAX_AUTO_RETRIES; i++) {
      expect(retry.schedule(attempt, scanning())).toBe(true);
    }
    expect(retry.schedule(attempt, scanning())).toBe(false);
    expect(retry.pending).toBe(false);
  });

  it('replaces a pending retry instead of stacking one beside it', () => {
    const retry = createShelfInitRetry();
    const first = vi.fn();
    const second = vi.fn();

    retry.schedule(first, scanning());
    retry.schedule(second, scanning());
    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS);

    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it('cancels a pending retry while keeping the spent budget', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    for (let i = 1; i < SHELF_INIT_MAX_AUTO_RETRIES; i++) {
      retry.schedule(attempt, scanning());
    }
    retry.cancel();
    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS);

    expect(attempt).not.toHaveBeenCalled();
    expect(retry.schedule(attempt, scanning())).toBe(false);
  });

  it('restores the full budget on reset', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    for (let i = 1; i < SHELF_INIT_MAX_AUTO_RETRIES; i++) {
      retry.schedule(attempt, scanning());
    }
    expect(retry.schedule(attempt, scanning())).toBe(false);

    retry.reset();
    expect(retry.schedule(attempt, scanning())).toBe(true);
  });

  it('cancels the pending retry on reset', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    retry.schedule(attempt, scanning());
    retry.reset();
    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS);

    expect(attempt).not.toHaveBeenCalled();
  });
});

// api/client.ts publishes no reference for a refusal carrying Retry-After,
// because the retry that hides it usually succeeds. The attempt that spends the
// budget is not hidden - the caller shows the failure - so the reference has to
// come back with it.
describe('the reference on the terminal attempt', () => {
  const { incident, dismissIncident } = useErrorIncident();

  beforeEach(() => {
    dismissIncident();
  });

  it('stays absent while a retry is still being scheduled', () => {
    const retry = createShelfInitRetry();

    expect(retry.schedule(vi.fn(), scanning())).toBe(true);
    expect(incident.value).toBe('');
  });

  it('reappears once the budget is spent', () => {
    const retry = createShelfInitRetry();
    for (let i = 0; i < SHELF_INIT_MAX_AUTO_RETRIES - 1; i += 1) {
      retry.schedule(vi.fn(), scanning());
    }

    expect(retry.schedule(vi.fn(), scanning())).toBe(false);
    expect(incident.value).toBe('K7MQ4XZB');
  });

  it('publishes nothing for a refusal that carries no reference', () => {
    const retry = createShelfInitRetry();
    for (let i = 0; i < SHELF_INIT_MAX_AUTO_RETRIES; i += 1) {
      retry.schedule(vi.fn(), new ApiError('initializing', { status: 503 }));
    }

    expect(incident.value).toBe('');
  });
});
