// The on-device reading-stats document. Every client (web, Wails desktop,
// Capacitor Android) persists this exact shape; only the storage location
// differs (see storage.ts). Reading time is per-device behavior and is never
// sent to the server.
//
// `shelves` is keyed the same way reading history is (see
// buildDeviceDocumentKey), so a multi-shelf — or repointed — client keeps
// independent totals. Each entry maps "YYYY-MM-DD" to the seconds read that
// day; the server's per-book breakdown is not kept, because nothing ever read
// it.

export const READING_STATS_DOCUMENT_VERSION = 1;

// The dashboard asks for the last 365 days. Keeping a little more than that
// means a day near the edge is never dropped early by a write that happens to
// land at a different hour than the query.
export const READING_STATS_RETENTION_DAYS = 400;

// Ported from the server implementation these replace: they keep a stuck timer
// or a clock jump from recording an implausible amount of reading.
export const MAX_SECONDS_PER_CALL = 120;
export const MAX_SECONDS_PER_DAY = 24 * 60 * 60;

export interface ReadingStatsDocument {
  version: number;
  /** key -> "YYYY-MM-DD" -> total seconds read that day. */
  shelves: Record<string, Record<string, number>>;
}

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

export function createReadingStatsDocument(): ReadingStatsDocument {
  return { version: READING_STATS_DOCUMENT_VERSION, shelves: {} };
}

function clamp(value: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(value, hi));
}

function isValidDateKey(value: string): boolean {
  if (!DATE_PATTERN.test(value)) {
    return false;
  }
  const parsed = new Date(`${value}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return false;
  }
  // Rejects the likes of 2026-02-31, which Date rolls forward rather than
  // refusing.
  return toIsoDate(parsed) === value;
}

/** Local-calendar ISO date, matching how the reader and dashboard build theirs. */
export function toIsoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function normalizeSeconds(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return 0;
  }
  return clamp(Math.round(value), 0, MAX_SECONDS_PER_DAY);
}

function normalizeDays(value: unknown): Record<string, number> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return {};
  }

  const days: Record<string, number> = {};
  for (const [date, seconds] of Object.entries(value as Record<string, unknown>)) {
    if (!isValidDateKey(date)) {
      continue;
    }
    const normalized = normalizeSeconds(seconds);
    if (normalized > 0) {
      days[date] = normalized;
    }
  }
  return days;
}

/**
 * Reads a stored document, tolerating anything that is not a document this
 * build understands — unparseable text, a wrong shape, or a version written by
 * a future build — by falling back to an empty default. Reading stats are
 * disposable convenience state, so a corrupt file must never surface as an
 * error on the dashboard or block the reader's heartbeat.
 */
export function parseReadingStatsDocument(text: string | null | undefined): ReadingStatsDocument {
  if (typeof text !== 'string' || text.trim() === '') {
    return createReadingStatsDocument();
  }

  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return createReadingStatsDocument();
  }

  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return createReadingStatsDocument();
  }

  const candidate = raw as Partial<ReadingStatsDocument>;
  if (candidate.version !== READING_STATS_DOCUMENT_VERSION) {
    return createReadingStatsDocument();
  }

  const shelves: Record<string, Record<string, number>> = {};
  if (
    typeof candidate.shelves === 'object' &&
    candidate.shelves !== null &&
    !Array.isArray(candidate.shelves)
  ) {
    for (const [key, value] of Object.entries(candidate.shelves)) {
      const days = normalizeDays(value);
      if (Object.keys(days).length > 0) {
        shelves[key] = days;
      }
    }
  }

  return { version: READING_STATS_DOCUMENT_VERSION, shelves };
}

export function serializeReadingStatsDocument(doc: ReadingStatsDocument): string {
  return JSON.stringify(doc);
}

/**
 * Per-day totals for [from, to] inclusive. Days with no recorded reading are
 * omitted, matching the shape the dashboard already expects.
 */
export function getReadingStatsRange(
  doc: ReadingStatsDocument,
  key: string,
  from: string,
  to: string
): Record<string, number> {
  const days = doc.shelves[key];
  if (!days || from > to) {
    return {};
  }

  const result: Record<string, number> = {};
  for (const [date, seconds] of Object.entries(days)) {
    // The keys are zero-padded ISO dates, so lexicographic comparison is
    // chronological and needs no Date parsing per entry.
    if (date >= from && date <= to && seconds > 0) {
      result[date] = seconds;
    }
  }
  return result;
}

/**
 * Adds `seconds` to the given day, clamped per call and per day, then drops
 * anything older than the retention window. Days in the future relative to
 * `now` are rejected outright: they can only come from a clock jump, and one
 * would otherwise sit in the document until retention caught up with it.
 *
 * Returns `doc` unchanged when there is nothing to record, so the store skips
 * the write.
 */
export function withReadingSeconds(
  doc: ReadingStatsDocument,
  key: string,
  date: string,
  seconds: number,
  now: Date = new Date()
): ReadingStatsDocument {
  const added = clamp(Math.round(seconds), 0, MAX_SECONDS_PER_CALL);
  const today = toIsoDate(now);
  if (added <= 0 || !key || !isValidDateKey(date) || date > today) {
    return doc;
  }

  const oldest = new Date(now);
  oldest.setHours(0, 0, 0, 0);
  oldest.setDate(oldest.getDate() - (READING_STATS_RETENTION_DAYS - 1));
  const oldestKey = toIsoDate(oldest);
  if (date < oldestKey) {
    return doc;
  }

  const previous = doc.shelves[key] ?? {};
  const days: Record<string, number> = {};
  for (const [existing, value] of Object.entries(previous)) {
    if (existing >= oldestKey && existing <= today) {
      days[existing] = value;
    }
  }
  days[date] = clamp((days[date] ?? 0) + added, 0, MAX_SECONDS_PER_DAY);

  return { ...doc, shelves: { ...doc.shelves, [key]: days } };
}
