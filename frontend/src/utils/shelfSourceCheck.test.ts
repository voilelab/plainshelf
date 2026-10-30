import { describe, expect, it } from 'vitest';
import { shelfSourceCheckMessageKey, shelfSourceNeedsSecret } from './shelfSourceCheck';

describe('shelfSourceCheckMessageKey', () => {
  it('tells a wrong token from no connection', () => {
    expect(shelfSourceCheckMessageKey({ code: 'SOURCE_UNAUTHORIZED', books: 0 })).toBe('shelfSource.unauthorized');
    expect(shelfSourceCheckMessageKey({ code: 'SOURCE_UNAVAILABLE', books: 0 })).toBe('shelfSource.unavailable');
    expect(shelfSourceCheckMessageKey({ code: 'NOT_A_SHELF', books: 0 })).toBe('shelfSource.notAShelf');
  });

  it('passes a shelf', () => {
    expect(shelfSourceCheckMessageKey({ books: 3 })).toBeNull();
    expect(shelfSourceCheckMessageKey({ code: '', books: 0 })).toBeNull();
  });

  it('reads an unknown code as no connection', () => {
    expect(shelfSourceCheckMessageKey({ code: 'SOMETHING_NEW', books: 0 })).toBe('shelfSource.unavailable');
  });
});

describe('shelfSourceNeedsSecret', () => {
  it('requires a token only for a PlainShelf export', () => {
    expect(shelfSourceNeedsSecret('plainshelf')).toBe(true);
    expect(shelfSourceNeedsSecret('webdav')).toBe(false);
  });

  it('requires a password for a WebDAV account', () => {
    expect(shelfSourceNeedsSecret('webdav', 'reader')).toBe(true);
    expect(shelfSourceNeedsSecret('webdav', '  ')).toBe(false);
  });
});
