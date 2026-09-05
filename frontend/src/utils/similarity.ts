import type { SimilarBookPair } from '@/api/books';

/**
 * The page asks the server once for every pair at or above the widest floor
 * ({@link SIMILARITY_FLOOR}); every tier and the subset toggle then narrow that
 * one result set in memory, so changing tiers never re-requests.
 */
export type SimilarityTierKey = 'near-identical' | 'same-book' | 'same-source';

interface SimilarityTier {
  key: SimilarityTierKey;
  labelKey: string;
  /** Minimum Jaccard a pair needs to appear in this tier. */
  threshold: number;
}

/**
 * The lowest Jaccard the page requests, so the one fetch covers all three tiers
 * and the slider's whole range.
 *
 * 0.15 looks low, but two unrelated novels of the same genre only score about
 * 0.02. Raising it silently drops the "two independent transcripts of one
 * audiobook" case — the low number is not a mistake.
 */
export const SIMILARITY_FLOOR = 0.15;

/** Widest last, so a UI can rely on each threshold being <= the one before. */
export const SIMILARITY_TIERS: readonly SimilarityTier[] = [
  { key: 'near-identical', labelKey: 'maintenance.similar.tiers.nearIdentical', threshold: 0.85 },
  { key: 'same-book', labelKey: 'maintenance.similar.tiers.sameBook', threshold: 0.45 },
  { key: 'same-source', labelKey: 'maintenance.similar.tiers.sameSource', threshold: SIMILARITY_FLOOR }
];

/** "Same book, different edition" — the tier the page opens on. */
export const DEFAULT_SIMILARITY_TIER: SimilarityTierKey = 'same-book';

/**
 * The minimum is pinned to {@link SIMILARITY_FLOOR}, not lower: the page fetches
 * once at the floor, so a threshold below it would silently under-report.
 */
export const SIMILARITY_SLIDER_MIN = SIMILARITY_FLOOR;
export const SIMILARITY_SLIDER_MAX = 1;
export const SIMILARITY_SLIDER_STEP = 0.01;

export function tierThreshold(key: SimilarityTierKey): number {
  return SIMILARITY_TIERS.find((tier) => tier.key === key)?.threshold ?? SIMILARITY_FLOOR;
}

interface SimilarityFilter {
  /** Active Jaccard floor, from the selected tier or the advanced slider. */
  threshold: number;
  /**
   * Orthogonal to the tier rather than a fourth, wider one: a book edited down
   * from another (25% truncated) scores only ~0.76 Jaccard, and someone hunting
   * for an edited-down copy should not have to loosen the overall similarity to
   * find it. When on, the tier threshold is ignored.
   */
  subsetOnly: boolean;
}

/**
 * Pure and total, so the page can call it in a computed. Loosening the threshold
 * never removes a pair, and the subset toggle is independent of it.
 */
export function filterSimilarPairs(
  pairs: readonly SimilarBookPair[],
  filter: SimilarityFilter
): SimilarBookPair[] {
  if (filter.subsetOnly) {
    return pairs.filter((pair) => pair.relation === 'subset');
  }
  return pairs.filter((pair) => pair.jaccard >= filter.threshold);
}

/**
 * The symmetric-difference ratio of two shingle sets,
 * |A△B| / (|A| + |B|) = (1 - J) / (1 + J), shown next to the advanced slider so
 * the abstract score reads as an amount of text. For feel, not an edit distance.
 */
export function approxDiffPer100Chars(jaccard: number): number {
  const clamped = Math.min(1, Math.max(0, jaccard));
  return Math.round((100 * (1 - clamped)) / (1 + clamped));
}

/**
 * The shingle size the server fingerprints with. {@link estimatedDiffRate}
 * inverts the shingling under it, so its answer is only right while this matches
 * the `k` in FingerprintStatus.algo. A constant rather than a per-shelf read
 * because the pair card shows one settled figure; revisit if k stops being 5.
 */
export const SIMILARITY_SHINGLE_K = 5;

/**
 * The Mash distance r = 1 - (2J/(1+J))^(1/k). More faithful to the eye than
 * {@link approxDiffPer100Chars}: one character edit disturbs up to k shingles,
 * so raw shingle disagreement overstates the character-level difference by
 * roughly a factor of k, which the k-th root divides out.
 *
 * Valid only when the differences are spread evenly — transcripts, OCR, typos.
 * For the `subset` case use {@link subsetShortfallPercent}, which measures the
 * missing span directly.
 */
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

/** {@link estimatedDiffRate} rendered as a rounded "about N per 100 characters". */
export function estimatedDiffPer100(jaccard: number, k: number = SIMILARITY_SHINGLE_K): number {
  return Math.round(estimatedDiffRate(jaccard, k) * 100);
}

/**
 * How much shorter the trimmed side of a `subset` pair is. Unlike the per-100
 * diff rate this stays meaningful when one book is a single contiguous cut of
 * the other. Returns 0 when the fuller side has no counted characters, so a
 * missing count never renders as a nonsense percentage.
 */
export function subsetShortfallPercent(normCharsA: number, normCharsB: number): number {
  const longer = Math.max(normCharsA, normCharsB);
  const shorter = Math.min(normCharsA, normCharsB);
  if (longer <= 0) {
    return 0;
  }
  return Math.round((1 - shorter / longer) * 100);
}
