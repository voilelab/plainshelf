import { describe, expect, it } from 'vitest';
import { mapWithConcurrency } from './concurrency';

describe('mapWithConcurrency', () => {
  it('preserves input order and never exceeds the limit', async () => {
    let inFlight = 0;
    let peak = 0;
    const result = await mapWithConcurrency([30, 10, 20, 0, 5], 2, async (ms) => {
      inFlight += 1;
      peak = Math.max(peak, inFlight);
      await new Promise((resolve) => setTimeout(resolve, ms));
      inFlight -= 1;
      return ms * 2;
    });

    expect(result).toEqual([60, 20, 40, 0, 10]);
    expect(peak).toBe(2);
  });

  it('returns an empty array for no items', async () => {
    expect(await mapWithConcurrency([], 3, async (x: number) => x)).toEqual([]);
  });
});
