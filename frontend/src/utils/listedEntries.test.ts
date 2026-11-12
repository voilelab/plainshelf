import { describe, expect, it } from 'vitest';
import { splitListedEntries } from './listedEntries';

describe('splitListedEntries', () => {
  it('lists every entry up to the limit', () => {
    const entries = Array.from({ length: 10 }, (_, i) => `f${i}`);
    expect(splitListedEntries(entries)).toEqual({ shown: entries, hidden: 0 });
  });

  it('replaces the last line with a count once over the limit', () => {
    const entries = Array.from({ length: 12 }, (_, i) => `f${i}`);
    expect(splitListedEntries(entries)).toEqual({ shown: entries.slice(0, 9), hidden: 3 });
  });
});
