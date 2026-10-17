/**
 * A virtual shelf's source as the add-shelf forms take it: another
 * PlainShelf's export, or any WebDAV server. Go owns the format
 * (shelf.SourceConf); these forms only collect the fields.
 */
export type ShelfSourceKind = 'plainshelf' | 'webdav';

/** What Go's source check answers; code is empty when the source is a shelf. */
export interface ShelfSourceCheck {
  code?: string;
  books: number;
}

const CHECK_MESSAGE_KEYS: Record<string, string> = {
  SOURCE_UNAUTHORIZED: 'shelfSource.unauthorized',
  SOURCE_UNAVAILABLE: 'shelfSource.unavailable',
  NOT_A_SHELF: 'shelfSource.notAShelf'
};

/** The i18n key explaining a failed check, or null when the source is a shelf. */
export function shelfSourceCheckMessageKey(check: ShelfSourceCheck): string | null {
  if (!check.code) {
    return null;
  }
  return CHECK_MESSAGE_KEYS[check.code] ?? 'shelfSource.unavailable';
}

/** A plainshelf export always needs its token; WebDAV may need none. */
export function shelfSourceNeedsSecret(kind: ShelfSourceKind): boolean {
  return kind === 'plainshelf';
}
