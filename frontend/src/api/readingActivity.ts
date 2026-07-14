import { buildShelfApiPath, fetchJson, isMockApiMode } from './client';

interface BackendReadingActivityDay {
  total_seconds: number;
}

interface BackendReadingActivityResponse {
  days: Record<string, BackendReadingActivityDay>;
  unit: string;
}

function delay<T>(value: T, ms = 240): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), ms);
  });
}

// Fixed seed for the mock reading-activity generator below. Keeping this a
// constant (rather than deriving from Date.now()) is what makes the mock
// data reproducible across reloads and testable: a test can recompute
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

// Exported so mock-mode verification can recompute expected values directly
// from a date string, without scraping computed styles from the DOM. Pure
// function of (dateISO, MOCK_SEED) — stable regardless of which day "today"
// happens to be, which is what lets a fixed streak/heatmap deterministically
// reappear across reloads.
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

function toIsoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function eachIsoDateInRange(from: string, to: string): string[] {
  const dates: string[] = [];
  const cursor = new Date(`${from}T00:00:00`);
  const end = new Date(`${to}T00:00:00`);
  if (Number.isNaN(cursor.getTime()) || Number.isNaN(end.getTime())) {
    return dates;
  }
  while (cursor <= end) {
    dates.push(toIsoDate(cursor));
    cursor.setDate(cursor.getDate() + 1);
  }
  return dates;
}

/**
 * Returns a flat map of "YYYY-MM-DD" -> total seconds read that day, for
 * [from, to] inclusive. Days with zero seconds are omitted (matches what the
 * server returns: handle_stats.go only includes days that have activity).
 */
export async function getReadingActivity(from: string, to: string): Promise<Record<string, number>> {
  if (isMockApiMode()) {
    const result: Record<string, number> = {};
    for (const date of eachIsoDateInRange(from, to)) {
      const seconds = mockReadingSecondsForDate(date);
      if (seconds > 0) {
        result[date] = seconds;
      }
    }
    return delay(result);
  }

  const query = `?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
  const response = await fetchJson<BackendReadingActivityResponse>(
    `${buildShelfApiPath('/reading_activity')}${query}`
  );

  const result: Record<string, number> = {};
  for (const [date, day] of Object.entries(response.days ?? {})) {
    result[date] = day.total_seconds;
  }
  return result;
}

export async function reportReadingActivity(bookID: string, seconds: number, date: string): Promise<void> {
  const trimmedBookID = bookID.trim();
  const roundedSeconds = Math.round(seconds);
  if (!trimmedBookID || roundedSeconds <= 0) {
    return;
  }

  if (isMockApiMode()) {
    await delay(undefined);
    return;
  }

  await fetchJson<void>(buildShelfApiPath('/reading_activity'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ book_id: trimmedBookID, seconds: roundedSeconds, date })
  });
}
