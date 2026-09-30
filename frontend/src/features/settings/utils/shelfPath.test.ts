import { describe, expect, it } from 'vitest';
import { isAbsoluteShelfPath } from './shelfPath';

describe('isAbsoluteShelfPath', () => {
  // The desktop app runs on all three platforms from one bundle, so the form
  // has to accept a Windows path while the test suite runs on Linux.
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
