// The on-device reading-history document. Every client (web, Wails desktop,
// Capacitor Android) persists this exact shape; only the storage location
// differs (see storage.ts). Reading history is per-device state and is never
// sent to the server.
//
// `shelves` is keyed by shelf id so a multi-shelf setup keeps independent
// histories, each ordered most-recently-read first.

const READ_HISTORY_DOCUMENT_VERSION = 1;
export const DEFAULT_READ_HISTORY_LIMIT = 100;

export interface ReadHistoryDocument {
  version: number;
  /** Maximum entries kept per shelf. 0 disables retaining history. */
  limit: number;
  shelves: Record<string, string[]>;
}

export function createReadHistoryDocument(): ReadHistoryDocument {
  return {
    version: READ_HISTORY_DOCUMENT_VERSION,
    limit: DEFAULT_READ_HISTORY_LIMIT,
    shelves: {}
  };
}

function normalizeReadHistoryLimit(value: unknown): number {
  if (typeof value === 'number' && Number.isInteger(value) && value >= 0) {
    return value;
  }
  return DEFAULT_READ_HISTORY_LIMIT;
}

function normalizeIDs(value: unknown, limit: number): string[] {
  if (!Array.isArray(value) || limit <= 0) {
    return [];
  }

  const ids: string[] = [];
  const seen = new Set<string>();
  for (const entry of value) {
    if (typeof entry !== 'string') {
      continue;
    }
    const trimmed = entry.trim();
    if (!trimmed || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    ids.push(trimmed);
    if (ids.length >= limit) {
      break;
    }
  }
  return ids;
}

/**
 * Reads a stored document, tolerating anything that is not a document this
 * build understands — unparseable text, a wrong shape, or a version written by
 * a future build — by falling back to an empty default. Reading history is
 * disposable convenience state, so a corrupt file must never surface as an
 * error in the reader or the history page.
 */
export function parseReadHistoryDocument(text: string | null | undefined): ReadHistoryDocument {
  if (typeof text !== 'string' || text.trim() === '') {
    return createReadHistoryDocument();
  }

  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return createReadHistoryDocument();
  }

  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return createReadHistoryDocument();
  }

  const candidate = raw as Partial<ReadHistoryDocument>;
  if (candidate.version !== READ_HISTORY_DOCUMENT_VERSION) {
    return createReadHistoryDocument();
  }

  const limit = normalizeReadHistoryLimit(candidate.limit);
  const shelves: Record<string, string[]> = {};
  if (typeof candidate.shelves === 'object' && candidate.shelves !== null && !Array.isArray(candidate.shelves)) {
    for (const [shelfID, value] of Object.entries(candidate.shelves)) {
      const ids = normalizeIDs(value, limit);
      if (ids.length > 0) {
        shelves[shelfID] = ids;
      }
    }
  }

  return { version: READ_HISTORY_DOCUMENT_VERSION, limit, shelves };
}

export function serializeReadHistoryDocument(doc: ReadHistoryDocument): string {
  return JSON.stringify(doc);
}

export function getShelfReadHistory(doc: ReadHistoryDocument, shelfID: string): string[] {
  return [...(doc.shelves[shelfID] ?? [])];
}

/** Moves `bookID` to the front of the shelf's history, deduplicated and trimmed. */
export function withReadEntry(
  doc: ReadHistoryDocument,
  shelfID: string,
  bookID: string
): ReadHistoryDocument {
  const trimmedID = bookID.trim();
  if (!trimmedID || doc.limit <= 0) {
    return doc;
  }

  const previous = doc.shelves[shelfID] ?? [];
  const next = [trimmedID, ...previous.filter((id) => id !== trimmedID)].slice(0, doc.limit);
  return { ...doc, shelves: { ...doc.shelves, [shelfID]: next } };
}

export function withClearedShelf(doc: ReadHistoryDocument, shelfID: string): ReadHistoryDocument {
  if (!(shelfID in doc.shelves)) {
    return doc;
  }

  const shelves = { ...doc.shelves };
  delete shelves[shelfID];
  return { ...doc, shelves };
}

/** Applies a new retention limit, trimming every shelf to match it. */
export function withLimit(doc: ReadHistoryDocument, limit: number): ReadHistoryDocument {
  const normalized = normalizeReadHistoryLimit(limit);
  const shelves: Record<string, string[]> = {};
  if (normalized > 0) {
    for (const [shelfID, ids] of Object.entries(doc.shelves)) {
      const trimmed = ids.slice(0, normalized);
      if (trimmed.length > 0) {
        shelves[shelfID] = trimmed;
      }
    }
  }

  return { ...doc, limit: normalized, shelves };
}
