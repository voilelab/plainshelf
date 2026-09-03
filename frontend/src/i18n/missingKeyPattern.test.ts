import { describe, expect, it } from 'vitest';
import en from './locales/en';
import { MISSING_KEY_PATTERN } from './missingKeyPattern';

// The end-to-end crawl that uses this pattern is only as good as the pattern
// itself, and an earlier version required two dots — which silently excluded
// every two-segment key, half of what it was written to catch. This case used
// to live in `locale.spec.ts` even though it starts no server and opens no
// browser; pinning both directions here is the same guard for none of the cost.

describe('MISSING_KEY_PATTERN', () => {
  it.each([
    'pagination.firstPage',
    'common.confirm',
    'layout.foldersNavLabel',
    'settings.shelves.idColumn',
    'notFound.title'
  ])('matches %s, which is what a missed lookup renders', (key) => {
    expect(key).toMatch(MISSING_KEY_PATTERN);
  });

  it.each(['hello.txt', 'notes.md', 'book.epub', 'Settings', 'No books yet.'])(
    'leaves %s alone, because a library renders filenames as titles',
    (text) => {
      expect(text).not.toMatch(MISSING_KEY_PATTERN);
    }
  );

  it('matches a key inside a sentence, the way one reaches the screen', () => {
    expect('Delete trash.confirmTitle now?').toMatch(MISSING_KEY_PATTERN);
  });

  it('covers every section the catalog actually has', () => {
    // The point of deriving the section list inside the module: a new catalog
    // section is guarded without anyone remembering to widen the pattern. Read
    // from the catalog here too, so this fails if the module ever goes back to
    // a hand-kept copy.
    const sections = Object.keys(en);

    expect(sections.length).toBeGreaterThan(10);
    for (const section of sections) {
      expect(`${section}.someKey`).toMatch(MISSING_KEY_PATTERN);
    }
  });
});
