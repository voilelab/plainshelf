// The on-device reading-progress document used by browser and Wails desktop
// clients. Android keeps its existing per-book progress.json files because
// they are already app-private and scoped per connection.

export const READING_PROGRESS_DOCUMENT_VERSION = 1;

export interface ReadingProgressDocument {
  version: number;
  /** device-document key -> book id -> UTF-16 character offset. */
  shelves: Record<string, Record<string, number>>;
}

export function createReadingProgressDocument(): ReadingProgressDocument {
  return { version: READING_PROGRESS_DOCUMENT_VERSION, shelves: {} };
}

function normalizeOffset(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value <= 0) {
    return null;
  }
  return value;
}

function normalizeShelf(value: unknown): Record<string, number> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return {};
  }

  const books: Record<string, number> = {};
  for (const [bookID, valueOffset] of Object.entries(value as Record<string, unknown>)) {
    const trimmedID = bookID.trim();
    const offset = normalizeOffset(valueOffset);
    if (trimmedID && offset !== null) {
      books[trimmedID] = offset;
    }
  }
  return books;
}

/**
 * Reading progress is convenience state. A missing, corrupt, wrongly shaped,
 * or future-version document therefore falls back to an empty document rather
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

  const shelves: Record<string, Record<string, number>> = {};
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
  return doc.shelves[shelfKey]?.[bookID] ?? 0;
}

export function withBookReadingOffset(
  doc: ReadingProgressDocument,
  shelfKey: string,
  bookID: string,
  offset: number
): ReadingProgressDocument {
  const trimmedShelfKey = shelfKey.trim();
  const trimmedBookID = bookID.trim();
  if (!trimmedShelfKey || !trimmedBookID) {
    return doc;
  }

  const normalizedOffset = normalizeOffset(offset);
  const previousShelf = doc.shelves[trimmedShelfKey] ?? {};

  // Zero (and any invalid input from a corrupt caller) means reset. Missing
  // entries already read as zero, so avoid keeping explicit zero records.
  if (normalizedOffset === null) {
    if (!(trimmedBookID in previousShelf)) {
      return doc;
    }
    const nextShelf = { ...previousShelf };
    delete nextShelf[trimmedBookID];
    const shelves = { ...doc.shelves };
    if (Object.keys(nextShelf).length === 0) {
      delete shelves[trimmedShelfKey];
    } else {
      shelves[trimmedShelfKey] = nextShelf;
    }
    return { ...doc, shelves };
  }

  if (previousShelf[trimmedBookID] === normalizedOffset) {
    return doc;
  }

  return {
    ...doc,
    shelves: {
      ...doc.shelves,
      [trimmedShelfKey]: { ...previousShelf, [trimmedBookID]: normalizedOffset }
    }
  };
}
