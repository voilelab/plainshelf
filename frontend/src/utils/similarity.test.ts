import { describe, expect, it } from 'vitest';

import type { SimilarBookPair, SimilarRelation } from '@/api/books';
import {
  approxDiffPer100Chars,
  DEFAULT_SIMILARITY_TIER,
  filterSimilarPairs,
  SIMILARITY_FLOOR,
  SIMILARITY_SLIDER_MIN,
  SIMILARITY_TIERS,
  tierThreshold
} from '@/utils/similarity';

function pair(jaccard: number, relation: SimilarRelation = 'near_identical'): SimilarBookPair {
  return {
    a: `a-${jaccard}-${relation}`,
    b: `b-${jaccard}-${relation}`,
    jaccard,
    containment_a: jaccard,
    containment_b: jaccard,
    norm_chars_a: 1000,
    norm_chars_b: 1000,
    relation
  };
}

// One pair per tier's canonical scenario, plus the truncated-25% subset case
// that the subset toggle is meant to reach without loosening the tier.
const truncatedSubset = pair(0.758, 'subset');
const pairs: SimilarBookPair[] = [
  pair(0.999, 'identical_after_normalize'),
  pair(0.905),
  pair(0.856),
  truncatedSubset,
  pair(0.501, 'same_source'),
  pair(0.292, 'same_source'),
  pair(0.16, 'same_source')
];

describe('similarity tiers', () => {
  it('exposes thresholds that widen monotonically and never dip below the floor', () => {
    const thresholds = SIMILARITY_TIERS.map((tier) => tier.threshold);
    for (let i = 1; i < thresholds.length; i += 1) {
      expect(thresholds[i]).toBeLessThanOrEqual(thresholds[i - 1]);
      expect(thresholds[i]).toBeGreaterThanOrEqual(SIMILARITY_FLOOR);
    }
  });

  it('opens on the same-book tier', () => {
    expect(DEFAULT_SIMILARITY_TIER).toBe('same-book');
    expect(tierThreshold('same-book')).toBe(0.45);
  });

  it('never lets the slider ask below the fetched floor (would under-report)', () => {
    // The page fetches once at the floor; a slider minimum beneath it would
    // filter a set the server never returned pairs below.
    expect(SIMILARITY_SLIDER_MIN).toBe(SIMILARITY_FLOOR);
  });
});

describe('filterSimilarPairs', () => {
  it('returns more pairs as the threshold loosens (result size is monotonic)', () => {
    const strict = filterSimilarPairs(pairs, { threshold: 0.85, subsetOnly: false });
    const mid = filterSimilarPairs(pairs, { threshold: 0.45, subsetOnly: false });
    const wide = filterSimilarPairs(pairs, { threshold: SIMILARITY_FLOOR, subsetOnly: false });

    expect(strict.length).toBeLessThanOrEqual(mid.length);
    expect(mid.length).toBeLessThanOrEqual(wide.length);

    // Loosening only adds; every stricter result stays in the looser one.
    expect(new Set(mid)).toEqual(new Set([...mid]));
    for (const p of strict) expect(mid).toContain(p);
    for (const p of mid) expect(wide).toContain(p);
  });

  it('keeps a pair once its Jaccard meets the threshold, inclusive', () => {
    const atBoundary = filterSimilarPairs([pair(0.45)], { threshold: 0.45, subsetOnly: false });
    expect(atBoundary).toHaveLength(1);
  });

  it('subset toggle is independent of the threshold: the truncated-25% pair shows even at the strictest tier', () => {
    // At the strictest tier its 0.758 Jaccard is filtered out entirely...
    const strict = filterSimilarPairs(pairs, { threshold: 0.85, subsetOnly: false });
    expect(strict).not.toContain(truncatedSubset);

    // ...but the subset toggle surfaces it regardless of the tier threshold.
    for (const threshold of [0.85, 0.45, SIMILARITY_FLOOR]) {
      const subsetOnly = filterSimilarPairs(pairs, { threshold, subsetOnly: true });
      expect(subsetOnly).toContain(truncatedSubset);
      expect(subsetOnly.every((p) => p.relation === 'subset')).toBe(true);
    }
  });
});

describe('approxDiffPer100Chars', () => {
  it('is 0 at a perfect match and grows as similarity drops', () => {
    expect(approxDiffPer100Chars(1)).toBe(0);
    expect(approxDiffPer100Chars(0.85)).toBeLessThan(approxDiffPer100Chars(0.45));
    expect(approxDiffPer100Chars(0.45)).toBeLessThan(approxDiffPer100Chars(SIMILARITY_FLOOR));
  });

  it('clamps out-of-range input rather than returning nonsense', () => {
    expect(approxDiffPer100Chars(2)).toBe(0);
    expect(approxDiffPer100Chars(-1)).toBe(100);
  });
});
