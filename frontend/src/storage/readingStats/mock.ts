
const MOCK_SEED = 20260713;

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

export function mockReadingSecondsForDate(dateISO: string): number {
  const activityRoll = hashToUnit(`${dateISO}|active`);
  if (activityRoll >= 0.68) {
    return 0;
  }

  const secondsRoll = hashToUnit(`${dateISO}|seconds`);
  const minSeconds = 5 * 60;
  const maxSeconds = 2 * 60 * 60;
  return Math.round(minSeconds + secondsRoll * (maxSeconds - minSeconds));
}
