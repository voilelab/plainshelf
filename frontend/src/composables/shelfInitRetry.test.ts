import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@/api/client';
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

describe('createShelfInitRetry', () => {
  it('runs the attempt after the shared delay and not before', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    expect(retry.schedule(attempt)).toBe(true);
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
      expect(retry.schedule(attempt)).toBe(true);
    }
    expect(retry.schedule(attempt)).toBe(false);
    expect(retry.pending).toBe(false);
  });

  it('replaces a pending retry instead of stacking one beside it', () => {
    const retry = createShelfInitRetry();
    const first = vi.fn();
    const second = vi.fn();

    retry.schedule(first);
    retry.schedule(second);
    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS);

    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it('cancels a pending retry while keeping the spent budget', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    for (let i = 1; i < SHELF_INIT_MAX_AUTO_RETRIES; i++) {
      retry.schedule(attempt);
    }
    retry.cancel();
    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS);

    expect(attempt).not.toHaveBeenCalled();
    expect(retry.schedule(attempt)).toBe(false);
  });

  it('restores the full budget on reset', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    for (let i = 1; i < SHELF_INIT_MAX_AUTO_RETRIES; i++) {
      retry.schedule(attempt);
    }
    expect(retry.schedule(attempt)).toBe(false);

    retry.reset();
    expect(retry.schedule(attempt)).toBe(true);
  });

  it('cancels the pending retry on reset', () => {
    const retry = createShelfInitRetry();
    const attempt = vi.fn();

    retry.schedule(attempt);
    retry.reset();
    vi.advanceTimersByTime(SHELF_INIT_RETRY_DELAY_MS);

    expect(attempt).not.toHaveBeenCalled();
  });
});
