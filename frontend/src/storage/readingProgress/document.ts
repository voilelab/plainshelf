// The on-device reading-progress document used by browser and Wails desktop
// clients (the standalone reader included). Android keeps its existing per-book
// progress.json files because they are already app-private and scoped per
// connection.
//
// Every entry carries the wall-clock time it was written so that the desktop app
// and the standalone reader — two processes sharing one reading_progress.json —
// can reconcile concurrent writes newest-wins per book instead of by namespace.
// A reset is a timestamped tombstone (offset 0), not a deletion, so it too wins
// by recency rather than being resurrected by an older advance. Callers still
// deal only in plain offsets; the timestamp is internal to this module.

// Bumped from 1 to 2 for the timestamped shape. A document of any other version
// is treated as absent (see parse): the app is pre-alpha, so untimestamped v1
// progress is discarded rather than migrated.
export const READING_PROGRESS_DOCUMENT_VERSION = 2;

/** A book's stored progress: a UTF-16 offset and the epoch ms it was written. */
export interface ProgressEntry {
  offset: number;
  at: number;
}

export interface ReadingProgressDocument {
  version: number;
  /** device-document key -> book id -> progress entry. */
  shelves: Record<string, Record<string, ProgressEntry>>;
}

export function createReadingProgressDocument(): ReadingProgressDocument {
  return { version: READING_PROGRESS_DOCUMENT_VERSION, shelves: {} };
}

/** A non-negative, safe-integer number, or null when the value is not one. */
function normalizeCount(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    return null;
  }
  return value;
}

// An entry is kept when its offset is a valid non-negative integer and it is
// meaningful: a positive offset, or a zero offset with a timestamp (a reset
// tombstone). A bare {offset:0, at:0} is nothing and is dropped. An unparseable
// timestamp is treated as unknown (0, the oldest) rather than dropping the entry.
function normalizeEntry(value: unknown): ProgressEntry | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null;
  }
  const record = value as Record<string, unknown>;
  const offset = normalizeCount(record.offset);
  if (offset === null) {
    return null;
  }
  const at = normalizeCount(record.at) ?? 0;
  if (offset === 0 && at === 0) {
    return null;
  }
  return { offset, at };
}

function normalizeShelf(value: unknown): Record<string, ProgressEntry> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return {};
  }

  const books: Record<string, ProgressEntry> = {};
  for (const [bookID, entryValue] of Object.entries(value as Record<string, unknown>)) {
    const trimmedID = bookID.trim();
    const entry = normalizeEntry(entryValue);
    if (trimmedID && entry !== null) {
      books[trimmedID] = entry;
    }
  }
  return books;
}

/**
 * Reading progress is convenience state. A missing, corrupt, wrongly shaped,
 * or other-version document therefore falls back to an empty document rather
 * than preventing the reader from opening.
 */
export function parseReadingProgressDocument(
  text: string | null | undefined
): ReadingProgressDocument {
  if (typeof text !== 'string' || text.trim() === '') {
    return createReadingProgressDocument();
  }

  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return createReadingProgressDocument();
  }

  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return createReadingProgressDocument();
  }

  const candidate = raw as Partial<ReadingProgressDocument>;
  if (candidate.version !== READING_PROGRESS_DOCUMENT_VERSION) {
    return createReadingProgressDocument();
  }

  const shelves: Record<string, Record<string, ProgressEntry>> = {};
  if (
    typeof candidate.shelves === 'object' &&
    candidate.shelves !== null &&
    !Array.isArray(candidate.shelves)
  ) {
    for (const [shelfKey, value] of Object.entries(candidate.shelves)) {
      const trimmedKey = shelfKey.trim();
      const books = normalizeShelf(value);
      if (trimmedKey && Object.keys(books).length > 0) {
        shelves[trimmedKey] = books;
      }
    }
  }

  return { version: READING_PROGRESS_DOCUMENT_VERSION, shelves };
}

export function serializeReadingProgressDocument(doc: ReadingProgressDocument): string {
  return JSON.stringify(doc);
}

export function getBookReadingOffset(
  doc: ReadingProgressDocument,
  shelfKey: string,
  bookID: string
): number {
  return doc.shelves[shelfKey]?.[bookID]?.offset ?? 0;
}

/**
 * The full stored entry (offset plus the epoch ms it was written) for a book, or
 * null when the book has no entry. Unlike getBookReadingOffset this also exposes
 * the timestamp, which a "last read" label needs; writers still go through the
 * offset helpers so the timestamp stays internal to writes.
 */
export function getBookReadingEntry(
  doc: ReadingProgressDocument,
  shelfKey: string,
  bookID: string
): ProgressEntry | null {
  return doc.shelves[shelfKey]?.[bookID] ?? null;
}

function setBookEntry(
  doc: ReadingProgressDocument,
  shelfKey: string,
  bookID: string,
  entry: ProgressEntry
): ReadingProgressDocument {
  return {
    ...doc,
    shelves: {
      ...doc.shelves,
      [shelfKey]: { ...(doc.shelves[shelfKey] ?? {}), [bookID]: entry }
    }
  };
}

/**
 * Records an offset for a book, stamped with `at` (defaulting to now) so a later
 * cross-process merge can pick the most recent write.
 *
 * A non-positive or invalid offset is a reset: it stores a zero-offset tombstone
 * carrying `at`, rather than deleting the entry, so the reset out-competes an
 * older advance the other process may hold. Resetting a book that has no entry at
 * all is a no-op — there is nothing to out-compete, and a tombstone would only be
 * noise. An unchanged offset is also a no-op, so re-reading the same position does
 * not churn the timestamp or force a write.
 */
export function withBookReadingOffset(
  doc: ReadingProgressDocument,
  shelfKey: string,
  bookID: string,
  offset: number,
  at: number = Date.now()
): ReadingProgressDocument {
  const trimmedShelfKey = shelfKey.trim();
  const trimmedBookID = bookID.trim();
  if (!trimmedShelfKey || !trimmedBookID) {
    return doc;
  }

  const previous = doc.shelves[trimmedShelfKey]?.[trimmedBookID];
  const normalizedOffset = normalizeCount(offset);

  // Reset: any non-positive or invalid offset. Missing entries already read as
  // zero, so only an existing, non-tombstone entry needs a tombstone written.
  if (normalizedOffset === null || normalizedOffset === 0) {
    if (!previous || previous.offset === 0) {
      return doc;
    }
    return setBookEntry(doc, trimmedShelfKey, trimmedBookID, { offset: 0, at });
  }

  if (previous && previous.offset === normalizedOffset) {
    return doc;
  }

  return setBookEntry(doc, trimmedShelfKey, trimmedBookID, { offset: normalizedOffset, at });
}
