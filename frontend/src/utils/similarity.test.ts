import { describe, expect, it } from 'vitest';

import type { SimilarBookPair, SimilarRelation } from '@/api/books';
import {
  approxDiffPer100Chars,
  DEFAULT_SIMILARITY_TIER,
  estimatedDiffPer100,
  estimatedDiffRate,
  filterSimilarPairs,
  SIMILARITY_FLOOR,
  SIMILARITY_SHINGLE_K,
  SIMILARITY_TIERS,
  subsetShortfallPercent,
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
});

describe('filterSimilarPairs', () => {
  it('returns more pairs as the threshold loosens (result size is monotonic)', () => {
    const strict = filterSimilarPairs(pairs, { threshold: 0.85, subsetOnly: false });
    const mid = filterSimilarPairs(pairs, { threshold: 0.45, subsetOnly: false });
    const wide = filterSimilarPairs(pairs, { threshold: SIMILARITY_FLOOR, subsetOnly: false });

    expect(strict.length).toBeLessThanOrEqual(mid.length);
    expect(mid.length).toBeLessThanOrEqual(wide.length);

    // Loosening only adds; every stricter result stays in the looser one.
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

describe('estimatedDiffRate', () => {
  // The conversion table the pair-card ticket is specified against, at n = 5.
  const table: [number, number][] = [
    [0.999, 0],
    [0.9, 1.1],
    [0.85, 1.7],
    [0.75, 3.0],
    [0.5, 7.8],
    [0.29, 14.8],
    [0.15, 23.6]
  ];

  it('reproduces the published conversion table to one decimal (k = 5)', () => {
    expect(SIMILARITY_SHINGLE_K).toBe(5);
    for (const [jaccard, percent] of table) {
      expect(estimatedDiffRate(jaccard) * 100).toBeCloseTo(percent, 1);
    }
  });

  it('divides shingle disagreement down: it reads far below the raw ratio', () => {
    // A single edit disturbs up to k shingles, so the Mash estimate must sit
    // well under approxDiffPer100Chars for the same Jaccard.
    expect(estimatedDiffPer100(0.85)).toBeLessThan(approxDiffPer100Chars(0.85));
    expect(estimatedDiffPer100(0.85)).toBe(2);
  });

  it('clamps out-of-range input to the closed [0, 1] rate', () => {
    expect(estimatedDiffRate(2)).toBe(0);
    expect(estimatedDiffRate(-1)).toBe(1);
  });
});

describe('subsetShortfallPercent', () => {
  it('is the fraction the shorter side is missing, order-independent', () => {
    // A copy trimmed to 75% of the fuller one is missing about 25%.
    expect(subsetShortfallPercent(750, 1000)).toBe(25);
    expect(subsetShortfallPercent(1000, 750)).toBe(25);
  });

  it('is 0 for equal lengths and for a missing (zero) fuller side', () => {
    expect(subsetShortfallPercent(1000, 1000)).toBe(0);
    expect(subsetShortfallPercent(0, 0)).toBe(0);
  });
});
