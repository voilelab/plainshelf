import type { SimilarBookPair } from '@/api/books';

/**
 * Similarity tiers, and the client-side filtering they drive.
 *
 * The similar-content page asks the server once for every pair at or above the
 * widest floor ({@link SIMILARITY_FLOOR}); every tier and the subset toggle then
 * narrow that one result set in memory, so changing tiers is instant and never
 * re-requests. The thresholds live here rather than in the component so the
 * page, the filter bar, and the tests all read the same numbers.
 */
export type SimilarityTierKey = 'near-identical' | 'same-book' | 'same-source';

interface SimilarityTier {
  key: SimilarityTierKey;
  labelKey: string;
  /** Minimum Jaccard a pair needs to appear in this tier. */
  threshold: number;
}

/**
 * The lowest Jaccard the page requests. Every tier's threshold is at or above
 * it, so the one fetch covers all three tiers and the slider's whole range.
 *
 * 0.15 looks low, but two unrelated novels of the same genre only score about
 * 0.02, so it still clears the background by a wide margin. Raising it silently
 * drops the "two independent transcripts of one audiobook" case — do not treat
 * the low number as a mistake. (See the source of truth in the server's
 * handle_fingerprints.go.)
 */
export const SIMILARITY_FLOOR = 0.15;

/**
 * The three tiers, widest last. Ordered so a UI can render them most-strict
 * first and rely on each threshold being <= the one before it.
 */
export const SIMILARITY_TIERS: readonly SimilarityTier[] = [
  { key: 'near-identical', labelKey: 'maintenance.similar.tiers.nearIdentical', threshold: 0.85 },
  { key: 'same-book', labelKey: 'maintenance.similar.tiers.sameBook', threshold: 0.45 },
  { key: 'same-source', labelKey: 'maintenance.similar.tiers.sameSource', threshold: SIMILARITY_FLOOR }
];

/** "Same book, different edition" — the tier the page opens on. */
export const DEFAULT_SIMILARITY_TIER: SimilarityTierKey = 'same-book';

/**
 * Advanced slider bounds. The minimum is pinned to {@link SIMILARITY_FLOOR}, not
 * lower: the page fetches once at the floor, so a threshold below it would filter
 * a set the server never returned pairs beneath and silently under-report. The
 * slider therefore cannot ask for less similarity than the one fetch can answer.
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
   * Subset toggle. Orthogonal to the tier rather than a fourth, wider tier: a
   * book edited down from another (e.g. 25% truncated) scores only ~0.76
   * Jaccard, so it would need the "same book" tier to surface — but someone
   * hunting for an edited-down copy should not have to loosen the overall
   * similarity to find it. When on, the tier threshold is ignored and only
   * pairs the server classified as `subset` are shown.
   */
  subsetOnly: boolean;
}

/**
 * Narrows the fetched pairs to the active filter. Pure and total, so the page
 * can call it in a computed and the tests can assert its guarantees directly:
 *
 * - loosening the threshold never removes a pair (result size is monotonic in
 *   the threshold), and
 * - the subset toggle is independent of the threshold.
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
 * A rough "per 100 characters, about N differ" readout for a Jaccard threshold,
 * shown next to the advanced slider so the abstract score reads as an amount of
 * text. Derived from the symmetric-difference ratio of two shingle sets,
 * |A△B| / (|A| + |B|) = (1 - J) / (1 + J); it is an approximation for feel, not
 * an exact edit distance. Monotonically decreasing in J, and 0 at J = 1.
 */
export function approxDiffPer100Chars(jaccard: number): number {
  const clamped = Math.min(1, Math.max(0, jaccard));
  return Math.round((100 * (1 - clamped)) / (1 + clamped));
}

/**
 * The shingle size (k) the server fingerprints with. {@link estimatedDiffRate}
 * inverts the shingling under this k, so the number it produces is only right
 * while it matches the server's ruleset (the `k` in FingerprintStatus.algo, from
 * handle_fingerprints.go). Kept as a constant rather than read per shelf because
 * the pair card shows one settled figure per pair, not a live reading of each
 * shelf's algorithm; revisit if the server's k ever stops being 5.
 */
export const SIMILARITY_SHINGLE_K = 5;

/**
 * Estimated fraction of characters that differ between two texts, given the
 * Jaccard of their k-shingle sets: the Mash distance r = 1 - (2J/(1+J))^(1/k).
 *
 * More faithful to the eye than {@link approxDiffPer100Chars}, which reports raw
 * shingle disagreement: a single character edit disturbs up to k shingles, so
 * the shingle-level difference overstates the character-level one by roughly a
 * factor of k, and dividing it out is what the k-th root does.
 *
 * Valid only when the differences are spread evenly through the text — speech
 * transcripts, OCR, scattered typos. For a book that is one contiguous slice of
 * another (the `subset` case) this number is meaningless; use
 * {@link subsetShortfallPercent}, which measures the missing span directly.
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
 * How much shorter the trimmed side of a `subset` pair is, as a percentage of
 * the fuller side's length: 1 - min/max over the two normalized character
 * counts. Unlike the per-100 diff rate this stays meaningful when one book is a
 * single contiguous cut of the other, which is exactly the subset case. Returns
 * 0 when the fuller side has no counted characters, so a missing count never
 * renders as a nonsense percentage.
 */
export function subsetShortfallPercent(normCharsA: number, normCharsB: number): number {
  const longer = Math.max(normCharsA, normCharsB);
  const shorter = Math.min(normCharsA, normCharsB);
  if (longer <= 0) {
    return 0;
  }
  return Math.round((1 - shorter / longer) * 100);
}
