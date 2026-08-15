// Deterministic reading activity for `VITE_USE_MOCK_API=true`, so the dev
// dashboard shows a plausible heatmap and streak without anyone having read
// anything.

// Fixed seed (rather than deriving from Date.now()) is what makes the mock data
// reproducible across reloads and testable: a test can recompute
// mockReadingSecondsForDate(date) for the same date string and get the same
// answer we did.
const MOCK_SEED = 20260713;

// Small FNV-1a-ish string hash mixed with MOCK_SEED, folded into [0, 1).
// Deterministic per input string; not cryptographic, just enough spread to
// look plausible in a calendar heatmap.
function hashToUnit(input: string): number {
  let hash = 2166136261 >>> 0;
  for (let i = 0; i < input.length; i += 1) {
    hash ^= input.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  hash ^= MOCK_SEED;
  hash = Math.imul(hash, 2654435761);
  hash >>>= 0;
  return hash / 4294967296;
}

/**
 * Pure function of (dateISO, MOCK_SEED) — stable regardless of which day
 * "today" happens to be, which is what lets a fixed streak/heatmap
 * deterministically reappear across reloads.
 */
export function mockReadingSecondsForDate(dateISO: string): number {
  // ~68% chance of not having read at all that day, so the generated year
  // has both visible streaks and gaps rather than being solid every day.
  const activityRoll = hashToUnit(`${dateISO}|active`);
  if (activityRoll >= 0.68) {
    return 0;
  }

  const secondsRoll = hashToUnit(`${dateISO}|seconds`);
  const minSeconds = 5 * 60;
  const maxSeconds = 2 * 60 * 60;
  return Math.round(minSeconds + secondsRoll * (maxSeconds - minSeconds));
}
