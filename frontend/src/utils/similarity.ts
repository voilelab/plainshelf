import type { SimilarBookPair } from '@/api/books';

export type SimilarityTierKey = 'near-identical' | 'same-book' | 'same-source';

interface SimilarityTier {
  key: SimilarityTierKey;
  labelKey: string;
  threshold: number;
}

export const SIMILARITY_FLOOR = 0.15;

export const SIMILARITY_TIERS: readonly SimilarityTier[] = [
  { key: 'near-identical', labelKey: 'maintenance.similar.tiers.nearIdentical', threshold: 0.85 },
  { key: 'same-book', labelKey: 'maintenance.similar.tiers.sameBook', threshold: 0.45 },
  { key: 'same-source', labelKey: 'maintenance.similar.tiers.sameSource', threshold: SIMILARITY_FLOOR }
];

export const DEFAULT_SIMILARITY_TIER: SimilarityTierKey = 'same-book';

export const SIMILARITY_SLIDER_MIN = SIMILARITY_FLOOR;
export const SIMILARITY_SLIDER_MAX = 1;
export const SIMILARITY_SLIDER_STEP = 0.01;

export function tierThreshold(key: SimilarityTierKey): number {
  return SIMILARITY_TIERS.find((tier) => tier.key === key)?.threshold ?? SIMILARITY_FLOOR;
}

interface SimilarityFilter {
  threshold: number;
  subsetOnly: boolean;
}

export function filterSimilarPairs(
  pairs: readonly SimilarBookPair[],
  filter: SimilarityFilter
): SimilarBookPair[] {
  if (filter.subsetOnly) {
    return pairs.filter((pair) => pair.relation === 'subset');
  }
  return pairs.filter((pair) => pair.jaccard >= filter.threshold);
}

export function approxDiffPer100Chars(jaccard: number): number {
  const clamped = Math.min(1, Math.max(0, jaccard));
  return Math.round((100 * (1 - clamped)) / (1 + clamped));
}

export const SIMILARITY_SHINGLE_K = 5;

export function estimatedDiffRate(jaccard: number, k: number = SIMILARITY_SHINGLE_K): number {
  const j = Math.min(1, Math.max(0, jaccard));
  if (j <= 0) {
    return 1;
  }
  if (j >= 1) {
    return 0;
  }
  return 1 - Math.pow((2 * j) / (1 + j), 1 / k);
}

export function estimatedDiffPer100(jaccard: number, k: number = SIMILARITY_SHINGLE_K): number {
  return Math.round(estimatedDiffRate(jaccard, k) * 100);
}

export function subsetShortfallPercent(normCharsA: number, normCharsB: number): number {
  const longer = Math.max(normCharsA, normCharsB);
  const shorter = Math.min(normCharsA, normCharsB);
  if (longer <= 0) {
    return 0;
  }
  return Math.round((1 - shorter / longer) * 100);
}
