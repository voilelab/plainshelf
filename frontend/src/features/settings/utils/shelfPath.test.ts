import { describe, expect, it } from 'vitest';
import { isAbsoluteShelfPath } from './shelfPath';

describe('isAbsoluteShelfPath', () => {
  it.each(['/mnt/archive', '/', 'C:\\books', 'c:/books', '\\\\server\\share', '  /mnt/archive  '])(
    'accepts %j',
    (dir) => {
      expect(isAbsoluteShelfPath(dir)).toBe(true);
    }
  );

  it.each(['', '   ', 'books', './books', '../books', 'books/archive', 'C:books'])(
    'refuses %j',
    (dir) => {
      expect(isAbsoluteShelfPath(dir)).toBe(false);
    }
  );
});
