export type ShelfSourceKind = 'plainshelf' | 'webdav';

export interface ShelfSourceCheck {
  code?: string;
  books: number;
}

const CHECK_MESSAGE_KEYS: Record<string, string> = {
  SOURCE_UNAUTHORIZED: 'shelfSource.unauthorized',
  SOURCE_UNAVAILABLE: 'shelfSource.unavailable',
  NOT_A_SHELF: 'shelfSource.notAShelf'
};

export function shelfSourceCheckMessageKey(check: ShelfSourceCheck): string | null {
  if (!check.code) {
    return null;
  }
  return CHECK_MESSAGE_KEYS[check.code] ?? 'shelfSource.unavailable';
}

export function shelfSourceNeedsSecret(kind: ShelfSourceKind, username = ''): boolean {
  return kind === 'plainshelf' || username.trim().length > 0;
}
